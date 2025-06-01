package main

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	authproto "registration-service/api/authproto/proto-generate"
	fileproto "registration-service/api/fileproto/proto-generate"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	authServiceAddr = "localhost:50051" // Убедитесь, что порт верный из вашего .env или docker-compose
	fileServiceAddr = "localhost:50052" // Убедитесь, что порт верный из вашего .env или docker-compose
)

func main() {
	// --- Подключение к AuthService ---
	authConn, err := grpc.Dial(authServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к AuthService: %v", err)
	}
	defer authConn.Close()
	authClient := authproto.NewAuthServiceClient(authConn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// --- Регистрация (или вход) ---
	// Пример регистрации нового пользователя
	// Для тестирования можно закомментировать регистрацию и использовать существующего пользователя для входа
	registerResp, err := authClient.Register(ctx, &authproto.RegisterRequest{
		Username: "testuser_client",
		Email:    "test_client@example.com",
		Password: "password123",
	})
	if err != nil {
		log.Printf("Ошибка регистрации (возможно, пользователь уже существует): %v", err)
		// Попробуем войти, если регистрация не удалась (например, пользователь уже существует)
	} else {
		log.Printf("Ответ регистрации: %s", registerResp.GetMessage())
	}

	// Вход для получения токена
	loginResp, err := authClient.Login(ctx, &authproto.LoginRequest{
		Username: "testuser_client", // Используйте того же пользователя, что и при регистрации
		Password: "password123",
	})
	if err != nil {
		log.Fatalf("Ошибка входа: %v", err)
	}
	log.Printf("Успешный вход. Токен получен.")
	accessToken := loginResp.GetToken()

	// --- Подключение к FileService ---
	fileConn, err := grpc.Dial(fileServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к FileService: %v", err)
	}
	defer fileConn.Close()
	fileClient := fileproto.NewFileServiceClient(fileConn)

	// --- Создание контекста с токеном авторизации ---
	md := metadata.Pairs("authorization", "Bearer "+accessToken)
	fileCtx := metadata.NewOutgoingContext(context.Background(), md)
	fileCtxTimeout, fileCancel := context.WithTimeout(fileCtx, time.Second*30) // Увеличим таймаут для файловых операций
	defer fileCancel()

	// --- Загрузка файла (пример) ---
	log.Println("Попытка загрузки файла...")
	// Создадим простой текстовый файл для загрузки
	dummyFileName := "test_upload.txt"
	dummyContent := []byte("Это тестовое содержимое файла для загрузки.")
	err = os.WriteFile(dummyFileName, dummyContent, 0644)
	if err != nil {
		log.Fatalf("Не удалось создать временный файл: %v", err)
	}
	defer os.Remove(dummyFileName) // Удаляем временный файл после

	stream, err := fileClient.UploadFile(fileCtxTimeout)
	if err != nil {
		log.Fatalf("Не удалось открыть стрим для UploadFile: %v", err)
	}

	// Сначала отправляем метаданные
	err = stream.Send(&fileproto.UploadFileRequest{
		Data: &fileproto.UploadFileRequest_Metadata{
			Metadata: &fileproto.FileMetadata{
				Name:        dummyFileName,
				ContentType: "text/plain",
			},
		},
	})
	if err != nil {
		log.Fatalf("Не удалось отправить метаданные файла: %v. Ошибка стрима: %v", err, stream.RecvMsg(nil))
	}

	// Затем отправляем содержимое файла по частям (чанками)
	chunkSize := 1024 // 1KB
	buffer := make([]byte, chunkSize)
	file, err := os.Open(dummyFileName)
	if err != nil {
		log.Fatalf("Не удалось открыть файл для чтения: %v", err)
	}
	defer file.Close()

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Не удалось прочитать чанк из файла: %v", err)
		}
		err = stream.Send(&fileproto.UploadFileRequest{
			Data: &fileproto.UploadFileRequest_Chunk{
				Chunk: buffer[:n],
			},
		})
		if err != nil {
			log.Fatalf("Не удалось отправить чанк файла: %v. Ошибка стрима: %v", err, stream.RecvMsg(nil))

		}
	}

	uploadResp, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Не удалось получить ответ после загрузки файла: %v", err)
	}
	log.Printf("Ответ UploadFile: ID файла - %s, Сообщение - %s", uploadResp.GetFileId(), uploadResp.GetMessage())
	uploadedFileId := uploadResp.GetFileId()

	// --- Получение списка файлов ---
	log.Println("Попытка получить список файлов...")
	listFilesResp, err := fileClient.ListFiles(fileCtxTimeout, &fileproto.ListFilesRequest{IncludeShared: false})
	if err != nil {
		log.Fatalf("Ошибка при получении списка файлов: %v", err)
	}
	log.Println("Список файлов:")
	for _, f := range listFilesResp.GetFiles() {
		log.Printf(" - ID: %s, Имя: %s, Размер: %d, Владелец: %t", f.GetFileId(), f.GetName(), f.GetSize(), f.GetIsOwner())
	}

	// --- Скачивание файла (пример) ---
	if uploadedFileId != "" {
		log.Printf("Попытка скачивания файла с ID: %s...", uploadedFileId)
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
		downloadedFileName := "downloaded_" + dummyFileName
		err = os.WriteFile(downloadedFileName, downloadedData, 0644)
		if err != nil {
			log.Fatalf("Не удалось сохранить скачанный файл: %v", err)
		}
		log.Printf("Файл %s успешно скачан и сохранен как %s. Размер: %d байт", dummyFileName, downloadedFileName, len(downloadedData))
		defer os.Remove(downloadedFileName)
	}

	// --- Получение информации о файле (GetFileInfo) ---
	if uploadedFileId != "" {
		log.Printf("Попытка получить информацию о файле с ID: %s...", uploadedFileId)
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
	newFileName := "renamed_" + dummyFileName
	if uploadedFileId != "" {
		log.Printf("Попытка переименовать файл с ID: %s в '%s'...", uploadedFileId, newFileName)
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
				log.Fatalf("Не удалось сохранить скачанный откаченный файл: %v", err)
			}
			log.Printf("Откаченный файл %s успешно скачан и сохранен как %s. Размер: %d байт", newFileName, revertedFileName, len(revertedData))
			defer os.Remove(revertedFileName)
		}
	}

	// --- Удаление файла (DeleteFile) --- TODO: Переместить в конец после всех тестов
	/*
		if uploadedFileId != "" {
			log.Printf("Попытка удалить файл с ID: %s...", uploadedFileId)
			_, err := fileClient.DeleteFile(fileCtxTimeout, &fileproto.DeleteFileRequest{FileId: uploadedFileId})
			if err != nil {
				log.Fatalf("Ошибка при удалении файла: %v", err)
			}
			log.Printf("Файл с ID: %s успешно удален.", uploadedFileId)

			// Попытка получить информацию об удаленном файле (должна быть ошибка)
			log.Printf("Попытка получить информацию об удаленном файле с ID: %s...", uploadedFileId)
			_, err = fileClient.GetFileInfo(fileCtxTimeout, &fileproto.GetFileInfoRequest{FileId: uploadedFileId})
			if err == nil {
				log.Fatalf("ОШИБКА: GetFileInfo для удаленного файла не вернул ошибку!")
			}
			log.Printf("Получена ожидаемая ошибка при запросе информации об удаленном файле: %v", err)
		}
	*/

	log.Println("Тестовый клиент завершил работу.")
}
