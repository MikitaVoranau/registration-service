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
)

func main() {
	fmt.Println("Клиент запущен...")

	// --- Подключение к AuthService ---
	fmt.Println("Подключение к AuthService...")
	authConn, err := grpc.Dial(authServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к AuthService: %v", err)
	}
	defer authConn.Close()
	authClient := authproto.NewAuthServiceClient(authConn)
	fmt.Println("Успешно подключено к AuthService.")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// --- Регистрация (или вход) ---
	fmt.Println("Попытка регистрации нового пользователя...")
	// Пример регистрации нового пользователя
	// Для тестирования можно закомментировать регистрацию и использовать существующего пользователя для входа
	registerResp, err := authClient.Register(ctx, &authproto.RegisterRequest{
		Username: "testuser_client",
		Email:    "test_client@example.com",
		Password: "password123",
	})
	if err != nil {
		log.Printf("Ошибка регистрации (возможно, пользователь уже существует): %v", err)
		fmt.Printf("Ошибка регистрации: %v\n", err)
		// Попробуем войти, если регистрация не удалась (например, пользователь уже существует)
	} else {
		log.Printf("Ответ регистрации: %s", registerResp.GetMessage())
		fmt.Printf("Ответ регистрации: %s\n", registerResp.GetMessage())
	}

	fmt.Println("Попытка входа пользователя...")
	// Вход для получения токена
	loginResp, err := authClient.Login(ctx, &authproto.LoginRequest{
		Username: "testuser_client", // Используйте того же пользователя, что и при регистрации
		Password: "password123",
	})
	if err != nil {
		log.Fatalf("Ошибка входа: %v", err)
	}
	log.Printf("Успешный вход. Токен получен.")
	fmt.Println("Успешный вход. Токен получен.")
	accessToken := loginResp.GetToken()

	// --- Подключение к FileService ---
	fmt.Println("Подключение к FileService...")
	fileConn, err := grpc.Dial(fileServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к FileService: %v", err)
	}
	defer fileConn.Close()
	fileClient := fileproto.NewFileServiceClient(fileConn)
	fmt.Println("Успешно подключено к FileService.")

	// --- Создание контекста с токеном авторизации ---
	md := metadata.Pairs("authorization", "Bearer "+accessToken)
	fileCtx := metadata.NewOutgoingContext(context.Background(), md)
	fileCtxTimeout, fileCancel := context.WithTimeout(fileCtx, time.Second*30) // Увеличим таймаут для файловых операций
	defer fileCancel()

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
}

// uploadFile загружает файл по указанному пути в хранилище
func uploadFile(ctx context.Context, client fileproto.FileServiceClient, filePath string, fileNameOnServer string) (string, error) {
	log.Printf("Загрузка файла: %s как %s", filePath, fileNameOnServer)
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("не удалось открыть файл %s: %w", filePath, err)
	}
	defer file.Close()

	stream, err := client.UploadFile(ctx)
	if err != nil {
		return "", fmt.Errorf("не удалось открыть стрим для UploadFile: %w", err)
	}

	// Сначала отправляем метаданные
	log.Println("Отправка метаданных файла...")
	err = stream.Send(&fileproto.UploadFileRequest{
		Data: &fileproto.UploadFileRequest_Metadata{
			Metadata: &fileproto.FileMetadata{
				Name:        fileNameOnServer,           // Используем переданное имя файла для сервера
				ContentType: "application/octet-stream", // Общий тип для файлов
			},
		},
	})
	if err != nil {
		// Попытка получить более детальную ошибку от сервера, если возможно
		if recvErr := stream.RecvMsg(nil); recvErr != nil && recvErr != io.EOF {
			return "", fmt.Errorf("не удалось отправить метаданные файла: %w (серверная ошибка: %v)", err, recvErr)
		}
		return "", fmt.Errorf("не удалось отправить метаданные файла: %w", err)
	}
	log.Println("Метаданные файла успешно отправлены.")

	// Затем отправляем содержимое файла по частям (чанками)
	log.Println("Отправка чанков файла...")
	chunkSize := 1024 // 1KB
	buffer := make([]byte, chunkSize)

	for {
		n, errRead := file.Read(buffer)
		if n > 0 {
			errSend := stream.Send(&fileproto.UploadFileRequest{
				Data: &fileproto.UploadFileRequest_Chunk{
					Chunk: buffer[:n],
				},
			})
			if errSend != nil {
				// Попытка получить более детальную ошибку от сервера
				if recvErr := stream.RecvMsg(nil); recvErr != nil && recvErr != io.EOF {
					return "", fmt.Errorf("не удалось отправить чанк файла: %w (серверная ошибка: %v)", errSend, recvErr)
				}
				return "", fmt.Errorf("не удалось отправить чанк файла: %w", errSend)
			}
		}
		if errRead == io.EOF {
			break
		}
		if errRead != nil {
			return "", fmt.Errorf("не удалось прочитать чанк из файла %s: %w", filePath, errRead)
		}
	}
	log.Println("Все чанки файла успешно отправлены.")

	uploadResp, err := stream.CloseAndRecv()
	if err != nil {
		return "", fmt.Errorf("не удалось получить ответ после загрузки файла: %w", err)
	}

	log.Printf("Ответ UploadFile: ID файла - %s, Сообщение - %s", uploadResp.GetFileId(), uploadResp.GetMessage())
	return uploadResp.GetFileId(), nil
}

// authenticateUser выполняет регистрацию (если необходимо) и вход пользователя, возвращая токен доступа.
// ... existing code ...
