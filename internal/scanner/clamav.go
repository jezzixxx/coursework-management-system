// internal/scanner/clamav.go
package scanner

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

var clamavAddr string

func init() {
	host := os.Getenv("CLAMAV_HOST")
	if host == "" {
		host = "clamav"
	}
	clamavAddr = host + ":3310"
}

func ScanForVirus(filePath string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	// Проверка на пустой файл
	stat, _ := f.Stat()
	if stat.Size() == 0 {
		return true, nil // Пустой файл считаем чистым
	}

	conn, err := net.DialTimeout("tcp", clamavAddr, 5*time.Second)
	if err != nil {
		return false, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	// 🔑 Команда с null-терминатором
	_, err = conn.Write([]byte("zINSTREAM\x00"))
	if err != nil {
		return false, fmt.Errorf("cmd: %w", err)
	}

	// 🔑 Стримим чанками по 1024 байта
	buf := make([]byte, 1024)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			header := make([]byte, 4)
			binary.BigEndian.PutUint32(header, uint32(n))
			if _, err := conn.Write(header); err != nil {
				return false, fmt.Errorf("hdr: %w", err)
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return false, fmt.Errorf("data: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return false, fmt.Errorf("read: %w", readErr)
		}
	}

	// 🔑 EOF-маркер
	_, err = conn.Write([]byte{0, 0, 0, 0})
	if err != nil {
		return false, fmt.Errorf("eof: %w", err)
	}

	// 🔑 Небольшая пауза, чтобы ClamAV успел ответить
	time.Sleep(100 * time.Millisecond)

	// 🔑 Читаем ответ
	respBuf := make([]byte, 4096)
	n, err := conn.Read(respBuf)
	if err != nil {
		return false, fmt.Errorf("read resp: %w", err)
	}
	result := string(respBuf[:n])

	// 🔑 Убираем ВСЕ служебные символы: пробелы, \n, \r, \t, \x00
	result = strings.Trim(result, " \n\r\t\x00")

	// 🔑 Парсим
	// Формат: "stream: OK" или "stream: <id>: <virus> FOUND"
	if strings.HasPrefix(result, "stream: ERROR:") {
		return false, fmt.Errorf("clamav: %s", strings.TrimPrefix(result, "stream: ERROR:"))
	}
	if strings.Contains(result, " FOUND") {
		parts := strings.Split(result, " FOUND")
		if len(parts) > 0 {
			last := parts[0]
			idx := strings.LastIndex(last, ": ")
			if idx != -1 && idx+2 < len(last) {
				threat := strings.TrimSpace(last[idx+2:])
				return false, fmt.Errorf("threat: %s", threat)
			}
		}
		return false, fmt.Errorf("threat: unknown")
	}
	// 🔑 Проверяем, что ответ содержит "OK" и не содержит "FOUND"
	if strings.Contains(result, "OK") && !strings.Contains(result, "FOUND") {
		return true, nil
	}

	return false, fmt.Errorf("unexpected: %q", result)
}
