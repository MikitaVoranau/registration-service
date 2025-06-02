package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	authproto "registration-service/api/authproto/proto-generate"
	fileproto "registration-service/api/fileproto/proto-generate"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	authServiceAddr = "localhost:50051" // Убедитесь, что порт верный из вашего .env или docker-compose
	fileServiceAddr = "localhost:50052" // Убедитесь, что порт верный из вашего .env или docker-compose

	user1Username = "user1_sharer"
	user1Password = "password123"
	user1Email    = "user1@example.com"

	user2Username = "user2_receiver"
	user2Password = "password456"
	user2Email    = "user2@example.com"

	permissionRead = 1 // Примерное значение для права на чтение
)

// authenticateUser handles registration (if necessary) and login, returning client, token, and userID.
func authenticateUser(ctx context.Context, authClient authproto.AuthServiceClient, username, email, password string) (string, int32, error) {
	fmt.Printf("Попытка регистрации пользователя %s...\\n", username)
	_, err := authClient.Register(ctx, &authproto.RegisterRequest{
		Username: username,
		Email:    email,
		Password: password,
	})
	if err != nil {
		// Проверяем, является ли ошибка "уже существует"
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.AlreadyExists {
			fmt.Printf("Пользователь %s уже существует, продолжаем вход.\\n", username)
		} else {
			log.Printf("Ошибка регистрации для %s (не 'AlreadyExists'): %v", username, err)
			// Не фатальная ошибка, если пользователь уже есть, логин должен сработать
		}
	} else {
		fmt.Printf("Пользователь %s успешно зарегистрирован.\\n", username)
	}

	fmt.Printf("Попытка входа пользователя %s...\\n", username)
	loginResp, err := authClient.Login(ctx, &authproto.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		return "", 0, fmt.Errorf("ошибка входа для %s: %w", username, err)
	}
	fmt.Printf("Успешный вход для %s. Токен получен. UserID: %d\\n", username, loginResp.GetUserId())
	return loginResp.GetToken(), int32(loginResp.GetUserId()), nil
}

func main() {
	fmt.Println("Клиент запущен...")

	// --- Подключение к AuthService ---
	fmt.Println("Подключение к AuthService...")
	authConn, err := grpc.Dial(authServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к AuthService: %v", err)
	}
	defer authConn.Close()
	authMasterClient := authproto.NewAuthServiceClient(authConn) // Назовем его master client
	fmt.Println("Успешно подключено к AuthService.")

	baseCtx, cancel := context.WithTimeout(context.Background(), time.Second*60) // Общий контекст
	defer cancel()

	// --- Аутентификация User 1 ---
	fmt.Println("\\n--- Аутентификация User 1 ---")
	accessTokenUser1, _, err := authenticateUser(baseCtx, authMasterClient, user1Username, user1Email, user1Password)
	if err != nil {
		log.Fatalf("Не удалось аутентифицировать User 1: %v", err)
	}

	// --- Аутентификация User 2 ---
	fmt.Println("\\n--- Аутентификация User 2 ---")
	accessTokenUser2, userIDUser2, err := authenticateUser(baseCtx, authMasterClient, user2Username, user2Email, user2Password)
	if err != nil {
		log.Fatalf("Не удалось аутентифицировать User 2: %v", err)
	}

	// --- Подключение к FileService (одно на всех, контексты будут разные) ---
	fmt.Println("\\n--- Подключение к FileService ---")
	fileConn, err := grpc.Dial(fileServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к FileService: %v", err)
	}
	defer fileConn.Close()
	fileMasterClient := fileproto.NewFileServiceClient(fileConn) // Назовем его master client
	fmt.Println("Успешно подключено к FileService.")

	// --- User 1: Загрузка файла ---
	fmt.Println("\\n--- User 1: Загрузка файла ---")
	mdUser1 := metadata.Pairs("authorization", "Bearer "+accessTokenUser1)
	fileCtxUser1 := metadata.NewOutgoingContext(baseCtx, mdUser1)

	fileNameForServerUser1 := "shared_by_user1.txt"
	localFilePathUser1 := "user1_sample.txt"
	fileContentUser1 := "Это тестовое содержимое от user1 для проверки шаринга."

	// Создадим user1_sample.txt
	if err := os.WriteFile(localFilePathUser1, []byte(fileContentUser1), 0644); err != nil {
		log.Fatalf("Не удалось создать %s: %v", localFilePathUser1, err)
	}
	defer os.Remove(localFilePathUser1)

	uploadedFileIdUser1, err := uploadFile(fileCtxUser1, fileMasterClient, localFilePathUser1, fileNameForServerUser1)
	if err != nil {
		log.Fatalf("User 1: Ошибка загрузки файла: %v", err)
	}
	fmt.Printf("User 1: Файл %s успешно загружен. ID файла: %s\\n", fileNameForServerUser1, uploadedFileIdUser1)

	// --- User 1: Проверка скачивания своего файла ---
	fmt.Println("\\n--- User 1: Проверка скачивания своего файла ---")
	downloadedDataUser1, err := downloadAndVerifyFile(fileCtxUser1, fileMasterClient, uploadedFileIdUser1, fileNameForServerUser1, []byte(fileContentUser1), "downloaded_by_user1_own.txt")
	if err != nil {
		log.Fatalf("User 1: Ошибка при скачивании или проверке своего файла: %v", err)
	}
	fmt.Printf("User 1: Успешно скачал и проверил свой файл. Размер: %d байт\\n", len(downloadedDataUser1))

	// --- User 1: Предоставление доступа User 2 ---
	fmt.Println("\\n--- User 1: Предоставление доступа User 2 к файлу ---")
	shareReq := &fileproto.SetFilePermissionsRequest{
		FileId: uploadedFileIdUser1,
		Permissions: []*fileproto.PermissionEntry{
			{
				UserId:         userIDUser2, // Делимся с User 2
				PermissionType: permissionRead,
			},
			// Важно: чтобы User1 не потерял доступ, нужно либо добавить его сюда же,
			// либо серверная логика SetFilePermissions должна быть достаточно умной,
			// чтобы не удалять права владельца. Предположим, что права владельца сохраняются.
			// Если нет, то нужно добавить:
			// { UserId: userIDUser1, PermissionType: permissionRead /* или другое право для владельца */ },
		},
	}
	_, err = fileMasterClient.SetFilePermissions(fileCtxUser1, shareReq)
	if err != nil {
		log.Fatalf("User 1: Ошибка при установке прав доступа для User 2: %v", err)
	}
	fmt.Printf("User 1: Успешно предоставлен доступ User 2 (ID: %d) к файлу %s\\n", userIDUser2, uploadedFileIdUser1)

	// --- User 2: Попытка скачивания файла, расшаренного User 1 ---
	fmt.Println("\\n--- User 2: Попытка скачивания файла от User 1 ---")
	mdUser2 := metadata.Pairs("authorization", "Bearer "+accessTokenUser2)
	fileCtxUser2 := metadata.NewOutgoingContext(baseCtx, mdUser2)

	downloadedDataUser2, err := downloadAndVerifyFile(fileCtxUser2, fileMasterClient, uploadedFileIdUser1, fileNameForServerUser1, []byte(fileContentUser1), "downloaded_by_user2_shared.txt")
	if err != nil {
		log.Fatalf("User 2: Ошибка при скачивании или проверке файла от User 1: %v", err)
	}
	fmt.Printf("User 2: Успешно скачал и проверил файл от User 1. Размер: %d байт\\n", len(downloadedDataUser2))

	// --- User 1: Удаление файла ---
	fmt.Println("\\n--- User 1: Удаление файла ---")
	deleteReq := &fileproto.DeleteFileRequest{FileId: uploadedFileIdUser1}
	_, err = fileMasterClient.DeleteFile(fileCtxUser1, deleteReq)
	if err != nil {
		log.Fatalf("User 1: Ошибка при удалении файла: %v", err)
	}
	fmt.Printf("User 1: Файл %s успешно удален.\\n", uploadedFileIdUser1)

	// --- User 2: Попытка скачать удаленный файл (ожидается ошибка) ---
	fmt.Println("\\n--- User 2: Попытка скачать удаленный/отозванный файл (ожидается ошибка) ---")
	_, err = downloadAndVerifyFile(fileCtxUser2, fileMasterClient, uploadedFileIdUser1, fileNameForServerUser1, []byte(fileContentUser1), "downloaded_by_user2_after_delete.txt")
	if err != nil {
		fmt.Printf("User 2: Ожидаемая ошибка при скачивании удаленного файла: %v\\n", err)
		// Проверяем, что это ошибка типа "не найдено" или "доступ запрещен"
		st, ok := status.FromError(err)
		if ok {
			if st.Code() == codes.NotFound || st.Code() == codes.PermissionDenied {
				fmt.Printf("User 2: Получена корректная ошибка gRPC: %s\\n", st.Code())
			} else {
				log.Fatalf("User 2: Получена НЕОЖИДАННАЯ ошибка gRPC при скачивании удаленного файла: %s - %s", st.Code(), st.Message())
			}
		} else {
			log.Fatalf("User 2: Получена НЕ gRPC ошибка при скачивании удаленного файла: %v", err)
		}
	} else {
		log.Fatalf("User 2: ОШИБКА! Удалось скачать файл, который должен был быть удален/недоступен.")
	}

	fmt.Println("\\n--- Все тесты завершены ---")

	// Старый код оставим закомментированным для справки, если нужно будет что-то из него взять
	// или для однопользовательского тестирования
	/*
		// --- Загрузка файла (пример) ---
		log.Println("Попытка загрузки файла...")
		fmt.Println("Попытка загрузки файла...")

		// Имя файла на сервере и путь к локальному файлу
		fileNameForServer := "uploaded_sample.txt"
		localFilePath := "sample.txt" // Убедитесь, что этот файл существует

		// Создадим sample.txt если он не существует для демонстрации
		if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
			log.Printf("Файл %s не найден, создаю его с тестовым содержимым.", localFilePath)
			dummyContent := []byte("Это содержимое файла sample.txt для загрузки.")
			err = os.WriteFile(localFilePath, dummyContent, 0644)
			if err != nil {
				log.Fatalf("Не удалось создать %s: %v", localFilePath, err)
			}
		}

		uploadedFileId, err := uploadFile(fileCtxTimeout, fileClient, localFilePath, fileNameForServer)
		if err != nil {
			log.Fatalf("Ошибка загрузки файла: %v", err)
		}
		log.Printf("Файл %s успешно загружен. ID файла: %s", localFilePath, uploadedFileId)
		fmt.Printf("Файл %s успешно загружен. ID файла: %s\n", localFilePath, uploadedFileId)

		// --- Получение списка файлов ---
		log.Println("Попытка получить список файлов...")
		fmt.Println("Попытка получить список файлов...")
		listFilesResp, err := fileClient.ListFiles(fileCtxTimeout, &fileproto.ListFilesRequest{IncludeShared: false})
		if err != nil {
			log.Fatalf("Ошибка при получении списка файлов: %v", err)
		}
		log.Println("Список файлов:")
		fmt.Println("Список файлов:")
		for _, f := range listFilesResp.GetFiles() {
			log.Printf(" - ID: %s, Имя: %s, Размер: %d, Владелец: %t", f.GetFileId(), f.GetName(), f.GetSize(), f.GetIsOwner())
			fmt.Printf(" - ID: %s, Имя: %s, Размер: %d, Владелец: %t\n", f.GetFileId(), f.GetName(), f.GetSize(), f.GetIsOwner())
		}

		// --- Скачивание файла (пример) ---
		if uploadedFileId != "" {
			log.Printf("Попытка скачивания файла с ID: %s...", uploadedFileId)
			fmt.Printf("Попытка скачивания файла с ID: %s...\\n", uploadedFileId)
			downloadStream, err := fileClient.DownloadFile(fileCtxTimeout, &fileproto.DownloadFileRequest{FileId: uploadedFileId})
			if err != nil {
				log.Fatalf("Не удалось начать скачивание файла: %v", err)
			}

			downloadedData := []byte{}
			for {
				resp, err := downloadStream.Recv()
				if err == io.EOF {
					break // Стрим завершен
				}
				if err != nil {
					log.Fatalf("Ошибка при получении чанка файла: %v", err)
				}
				downloadedData = append(downloadedData, resp.GetChunk()...)
			}
			downloadedFileName := "downloaded_" + fileNameForServer // Используем имя файла, которое было на сервере
			err = os.WriteFile(downloadedFileName, downloadedData, 0644)
			if err != nil {
				log.Fatalf("Не удалось сохранить скачанный файл: %v", err)
			}
			log.Printf("Файл %s успешно скачан и сохранен как %s. Размер: %d байт", fileNameForServer, downloadedFileName, len(downloadedData))
			fmt.Printf("Файл %s успешно скачан и сохранен как %s. Размер: %d байт\\n", fileNameForServer, downloadedFileName, len(downloadedData))

			// Проверка содержимого скачанного файла
			// Сначала прочитаем оригинальный файл для сравнения
			originalContent, err := os.ReadFile(localFilePath)
			if err != nil {
				log.Fatalf("Не удалось прочитать оригинальный файл %s для сравнения: %v", localFilePath, err)
			}
			if !bytes.Equal(downloadedData, originalContent) {
				log.Fatalf("ОШИБКА: Содержимое скачанного файла не совпадает с оригиналом! Ожидалось: %s, Получено: %s", string(originalContent), string(downloadedData))
			}
			fmt.Println("Содержимое скачанного файла успешно проверено.")
			defer os.Remove(downloadedFileName)
		}

		// --- Получение информации о файле (GetFileInfo) ---
		if uploadedFileId != "" {
			log.Printf("Попытка получить информацию о файле с ID: %s...", uploadedFileId)
			fmt.Printf("Попытка получить информацию о файле с ID: %s...\n", uploadedFileId)
			fileInfoResp, err := fileClient.GetFileInfo(fileCtxTimeout, &fileproto.GetFileInfoRequest{FileId: uploadedFileId})
			if err != nil {
				log.Fatalf("Ошибка при получении информации о файле: %v", err)
			}
			if fileInfoResp != nil && fileInfoResp.File != nil {
				log.Printf("Информация о файле: ID: %s, Имя: %s, Размер: %d, Версия: %d, ContentType: %s, Владелец: %t",
					fileInfoResp.File.GetFileId(),
					fileInfoResp.File.GetName(),
					fileInfoResp.File.GetSize(),
					fileInfoResp.File.GetVersion(),
					fileInfoResp.File.GetContentType(),
					fileInfoResp.File.GetIsOwner(),
				)
			} else {
				log.Printf("GetFileInfo вернул пустой ответ или пустой FileInfo.")
			}
		}

		// --- Переименование файла (RenameFile) ---
		newFileName := "renamed_" + fileNameForServer // Используем имя файла, которое было на сервере
		if uploadedFileId != "" {
			log.Printf("Попытка переименовать файл с ID: %s в '%s'...", uploadedFileId, newFileName)
			fmt.Printf("Попытка переименовать файл с ID: %s в '%s'...!\\n", uploadedFileId, newFileName)
			_, err := fileClient.RenameFile(fileCtxTimeout, &fileproto.RenameFileRequest{FileId: uploadedFileId, NewName: newFileName})
			if err != nil {
				log.Fatalf("Ошибка при переименовании файла: %v", err)
			}
			log.Printf("Файл успешно переименован в '%s'.", newFileName)

			// Проверим имя файла после переименования через GetFileInfo
			log.Printf("Повторное получение информации о файле с ID: %s для проверки имени...", uploadedFileId)
			renamedFileInfoResp, err := fileClient.GetFileInfo(fileCtxTimeout, &fileproto.GetFileInfoRequest{FileId: uploadedFileId})
			if err != nil {
				log.Fatalf("Ошибка при получении информации о переименованном файле: %v", err)
			}
			if renamedFileInfoResp != nil && renamedFileInfoResp.File != nil {
				log.Printf("Информация о переименованном файле: ID: %s, Имя: %s, Размер: %d",
					renamedFileInfoResp.File.GetFileId(),
					renamedFileInfoResp.File.GetName(),
					renamedFileInfoResp.File.GetSize(),
				)
				if renamedFileInfoResp.File.GetName() != newFileName {
					log.Fatalf("ОШИБКА: Имя файла после переименования ('%s') не совпадает с ожидаемым ('%s')!", renamedFileInfoResp.File.GetName(), newFileName)
				}
			} else {
				log.Printf("GetFileInfo для переименованного файла вернул пустой ответ.")
			}
		}

		// --- Получение версий файла (GetFileVersions) ---
		var initialVersionNumber uint32 = 0
		if uploadedFileId != "" {
			log.Printf("Попытка получить версии файла с ID: %s...", uploadedFileId)
			fmt.Printf("Попытка получить версии файла с ID: %s...\n", uploadedFileId)
			versionsResp, err := fileClient.GetFileVersions(fileCtxTimeout, &fileproto.GetFileVersionsRequest{FileId: uploadedFileId})
			if err != nil {
				log.Fatalf("Ошибка при получении версий файла: %v", err)
			}
			log.Printf("Версии файла (ID: %s):", uploadedFileId)
			for _, v := range versionsResp.GetVersions() {
				log.Printf("  - Версия: %d, Размер: %d, Создана: %s", v.GetVersionNumber(), v.GetSize(), time.Unix(v.GetCreatedAt(), 0).Format(time.RFC3339))
				if v.GetVersionNumber() > initialVersionNumber { // Сохраняем номер последней (или единственной) версии
					initialVersionNumber = v.GetVersionNumber()
				}
			}
			if initialVersionNumber == 0 && len(versionsResp.GetVersions()) > 0 {
				// Если вдруг initialVersionNumber не обновился, но версии есть, берем первую попавшуюся
				initialVersionNumber = versionsResp.GetVersions()[0].GetVersionNumber()
			} else if len(versionsResp.GetVersions()) == 0 {
				log.Printf("Для файла %s не найдено версий.", uploadedFileId)
				// В этом случае Revert не имеет смысла, можно или пропустить, или ожидать ошибку
			}
		}

		// --- Откат к версии файла (RevertFileVersion) ---
		var revertedFileId string
		if uploadedFileId != "" && initialVersionNumber > 0 {
			log.Printf("Попытка откатить файл ID: %s к версии %d...", uploadedFileId, initialVersionNumber)
			fmt.Printf("Попытка откатить файл ID: %s к версии %d...\n", uploadedFileId, initialVersionNumber)
			revertResp, err := fileClient.RevertFileVersion(fileCtxTimeout, &fileproto.RevertFileRequest{FileId: uploadedFileId, Version: initialVersionNumber})
			if err != nil {
				log.Fatalf("Ошибка при откате файла: %v", err)
			}
			if revertResp.GetSuccess() {
				revertedFileId = revertResp.GetNewFileId()
				log.Printf("Файл успешно откачен. ID новой версии (или тот же, если логика не создает новый ID при реверте к последней): %s", revertedFileId)
			} else {
				log.Fatalf("Откат файла не удался (success: false).")
			}

			// Снова получаем версии, чтобы увидеть новую версию
			log.Printf("Повторное получение версий файла с ID: %s после отката...", uploadedFileId)
			versionsAfterRevertResp, err := fileClient.GetFileVersions(fileCtxTimeout, &fileproto.GetFileVersionsRequest{FileId: uploadedFileId})
			if err != nil {
				log.Fatalf("Ошибка при получении версий файла после отката: %v", err)
			}
			log.Printf("Версии файла (ID: %s) после отката:", uploadedFileId)
			var latestVersionAfterRevert uint32 = 0
			for _, v := range versionsAfterRevertResp.GetVersions() {
				log.Printf("  - Версия: %d, Размер: %d, Создана: %s", v.GetVersionNumber(), v.GetSize(), time.Unix(v.GetCreatedAt(), 0).Format(time.RFC3339))
				if v.GetVersionNumber() > latestVersionAfterRevert {
					latestVersionAfterRevert = v.GetVersionNumber()
				}
			}
			// Тут можно добавить проверку, что latestVersionAfterRevert > initialVersionNumber
			// и что revertedFileId (если он меняется) соответствует новому файлу/версии

			// Попробуем скачать последнюю версию после отката
			if latestVersionAfterRevert > 0 {
				// Важно: RevertFileVersion в вашем FileService создает НОВУЮ ВЕРСИЮ исходного файла,
				// ID самого файла (File.ID) при этом не меняется. CurrentVersion в таблице files обновляется.
				log.Printf("Попытка скачивания файла ID: %s (который теперь является версией %d)...", uploadedFileId, latestVersionAfterRevert)
				downloadStreamReverted, err := fileClient.DownloadFile(fileCtxTimeout, &fileproto.DownloadFileRequest{FileId: uploadedFileId})
				if err != nil {
					log.Fatalf("Не удалось начать скачивание откаченной версии файла: %v", err)
				}
				revertedData := []byte{}
				for {
					respStream, errStream := downloadStreamReverted.Recv()
					if errStream == io.EOF {
						break
					}
					if errStream != nil {
						log.Fatalf("Ошибка при получении чанка откаченного файла: %v", errStream)
					}
					revertedData = append(revertedData, respStream.GetChunk()...)
				}
				revertedFileName := "reverted_" + newFileName // Используем newFileName, т.к. файл был переименован
				err = os.WriteFile(revertedFileName, revertedData, 0644)
				if err != nil {
					log.Fatalf("Не удалось сохранить откаченный файл: %v", err)
				}
				log.Printf("Файл %s (версия 1) успешно скачан и сохранен как %s. Размер: %d байт", newFileName, revertedFileName, len(revertedData))

				// Проверка содержимого откаченного файла (должен совпадать с первоначальным)
				originalContentToCompare, err := os.ReadFile(localFilePath) // Считываем оригинальный файл еще раз для сравнения
				if err != nil {
					log.Fatalf("Не удалось прочитать оригинальный файл %s для сравнения после отката: %v", localFilePath, err)
				}
				if !bytes.Equal(revertedData, originalContentToCompare) {
					log.Fatalf("ОШИБКА: Содержимое откаченного файла не совпадает с оригиналом (v1)! Ожидалось: %s, Получено: %s", string(originalContentToCompare), string(revertedData))
				}
				log.Println("Содержимое откаченного файла успешно проверено (v1).")
				defer os.Remove(revertedFileName)
			}
		}

		// --- Удаление файла (DeleteFile) ---
		if uploadedFileId != "" {
			log.Printf("Попытка удалить файл с ID: %s...", uploadedFileId)
			fmt.Printf("Попытка удалить файл с ID: %s...\n", uploadedFileId)
			_, err := fileClient.DeleteFile(fileCtxTimeout, &fileproto.DeleteFileRequest{FileId: uploadedFileId})
			if err != nil {
				log.Fatalf("Ошибка при удалении файла: %v", err)
			}
			log.Printf("Файл с ID: %s успешно удален.", uploadedFileId)
			fmt.Printf("Файл с ID: %s успешно удален.\n", uploadedFileId)

			// Попытка получить информацию об удаленном файле (должна быть ошибка)
			log.Printf("Попытка получить информацию об удаленном файле с ID: %s...", uploadedFileId)
			fmt.Printf("Попытка получить информацию об удаленном файле с ID: %s...\n", uploadedFileId)
			_, err = fileClient.GetFileInfo(fileCtxTimeout, &fileproto.GetFileInfoRequest{FileId: uploadedFileId})
			if err == nil {
				log.Fatalf("ОШИБКА: GetFileInfo для удаленного файла не вернул ошибку!")
			}
			st, ok := status.FromError(err)
			if ok && (st.Code() == codes.NotFound || st.Code() == codes.Internal) { // Сервер может вернуть Internal, если GetFileByID возвращает ошибку до проверки прав
				log.Printf("Получена ожидаемая ошибка '%s' при запросе информации об удаленном файле: %v", st.Code(), err)
				fmt.Printf("Получена ожидаемая ошибка '%s' при запросе информации об удаленном файле: %v\n", st.Code(), err)
			} else {
				log.Fatalf("ОШИБКА: GetFileInfo для удаленного файла вернул неожиданную ошибку или не ошибку gRPC: %v", err)
			}

			// Проверка отсутствия файла в списке
			log.Println("Попытка получить список файлов после удаления...")
			fmt.Println("Попытка получить список файлов после удаления...")
			listFilesAfterDeleteResp, errList := fileClient.ListFiles(fileCtxTimeout, &fileproto.ListFilesRequest{IncludeShared: false})
			if errList != nil {
				log.Fatalf("Ошибка при получении списка файлов после удаления: %v", errList)
			}
			foundDeletedFile := false
			for _, f := range listFilesAfterDeleteResp.GetFiles() {
				if f.GetFileId() == uploadedFileId {
					foundDeletedFile = true
					break
				}
			}
			if foundDeletedFile {
				log.Fatalf("ОШИБКА: Удаленный файл %s все еще присутствует в списке файлов!", uploadedFileId)
			}
			fmt.Printf("Удаленный файл %s не найден в списке файлов, как и ожидалось.\n", uploadedFileId)
		}

		fmt.Println("Все тесты файловых операций завершены.")
	*/
}

// downloadAndVerifyFile - вспомогательная функция для скачивания и проверки файла
func downloadAndVerifyFile(ctx context.Context, client fileproto.FileServiceClient, fileID, originalName string, originalContent []byte, localSaveName string) ([]byte, error) {
	fmt.Printf("Попытка скачивания файла с ID: %s (будет сохранен как %s)...\\n", fileID, localSaveName)
	downloadStream, err := client.DownloadFile(ctx, &fileproto.DownloadFileRequest{FileId: fileID})
	if err != nil {
		return nil, fmt.Errorf("не удалось начать скачивание файла %s: %w", fileID, err)
	}

	downloadedData := []byte{}
	for {
		resp, err := downloadStream.Recv()
		if err == io.EOF {
			break // Стрим завершен
		}
		if err != nil {
			// Оборачиваем ошибку, чтобы сохранить детали gRPC статуса
			return nil, fmt.Errorf("ошибка при получении чанка файла %s: %w", fileID, err)
		}
		downloadedData = append(downloadedData, resp.GetChunk()...)
	}

	if localSaveName != "" { // Если имя для сохранения передано
		err = os.WriteFile(localSaveName, downloadedData, 0644)
		if err != nil {
			log.Printf("Предупреждение: Не удалось сохранить скачанный файл %s как %s: %v", fileID, localSaveName, err)
			// Не прерываем выполнение, т.к. основное - это проверка содержимого
		} else {
			fmt.Printf("Файл %s успешно скачан и сохранен как %s. Размер: %d байт\\n", fileID, localSaveName, len(downloadedData))
			defer os.Remove(localSaveName) // Удаляем временный файл после проверки
		}
	}

	// Проверка содержимого скачанного файла
	if !bytes.Equal(downloadedData, originalContent) {
		// Не будем выводить всё содержимое в лог, если оно большое
		errMsg := fmt.Sprintf("ОШИБКА: Содержимое скачанного файла %s не совпадает с оригиналом! Ожидался размер %d, получен %d.", fileID, len(originalContent), len(downloadedData))
		if len(originalContent) < 200 && len(downloadedData) < 200 { // Выводим содержимое только если оно не слишком большое
			errMsg += fmt.Sprintf(" Ожидалось: '%s', Получено: '%s'", string(originalContent), string(downloadedData))
		}
		return nil, fmt.Errorf(errMsg)
	}
	fmt.Printf("Содержимое скачанного файла %s успешно проверено.\\n", fileID)
	return downloadedData, nil
}

// uploadFile (новая версия, используем переданный клиент и контекст)
func uploadFile(ctx context.Context, client fileproto.FileServiceClient, localFilePath string, fileNameOnServer string) (string, error) {
	fmt.Printf("Открытие локального файла %s для загрузки как %s...\\n", localFilePath, fileNameOnServer)
	file, err := os.Open(localFilePath)
	if err != nil {
		return "", fmt.Errorf("не удалось открыть файл %s: %w", localFilePath, err)
	}
	defer file.Close()

	stream, err := client.UploadFile(ctx)
	if err != nil {
		return "", fmt.Errorf("не удалось начать стрим загрузки: %w", err)
	}

	// Отправка метаданных файла
	metadata := &fileproto.FileMetadata{
		Name:        fileNameOnServer,
		ContentType: "text/plain", // Можно определять на основе расширения файла
	}
	req := &fileproto.UploadFileRequest{
		Data: &fileproto.UploadFileRequest_Metadata{Metadata: metadata},
	}
	if err := stream.Send(req); err != nil {
		return "", fmt.Errorf("не удалось отправить метаданные файла: %w", err)
	}
	fmt.Println("Метаданные файла отправлены.")

	// Отправка содержимого файла чанками
	buf := make([]byte, 1024) // Буфер для чтения
	totalBytesSent := 0
	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break // Файл полностью прочитан
		}
		if err != nil {
			return "", fmt.Errorf("ошибка чтения файла %s: %w", localFilePath, err)
		}
		if n > 0 {
			chunkReq := &fileproto.UploadFileRequest{
				Data: &fileproto.UploadFileRequest_Chunk{Chunk: buf[:n]},
			}
			if err := stream.Send(chunkReq); err != nil {
				return "", fmt.Errorf("не удалось отправить чанк файла: %w", err)
			}
			totalBytesSent += n
			// fmt.Printf("Отправлен чанк размером %d байт. Всего отправлено: %d\\n", n, totalBytesSent)
		}
	}
	fmt.Printf("Все чанки файла отправлены. Общий размер: %d байт.\\n", totalBytesSent)

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return "", fmt.Errorf("ошибка при закрытии стрима и получении ответа: %w", err)
	}

	fmt.Printf("Ответ от сервера после загрузки: ID файла - %s, Сообщение - %s\\n", resp.GetFileId(), resp.GetMessage())
	return resp.GetFileId(), nil
}
