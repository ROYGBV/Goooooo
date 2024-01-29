package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

func makeRequestHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка на то, отправлен ли POST запрос от клиента
	if r.Method != http.MethodPost {
		http.Error(w, "Поддерживаются только POST запросы", http.StatusMethodNotAllowed)
		return
	}

	// Проверка тела запроса на возможность считать его
	rBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}

	// Извлечение параметров из тела запроса
	var decodedData map[string]interface{}
	err = json.Unmarshal(rBody, &decodedData)
	if err != nil {
		http.Error(w, "Ошибка извлечения параметров из тела запроса", http.StatusBadRequest)
		return
	}
	url, urlExists := decodedData["url"].(string)
	method, methodExists := decodedData["method"].(string)
	jsonMessageStr, messageExists := decodedData["body"].(string)

	// Проверка наличия url и method
	if !urlExists {
		http.Error(w, "Отсутствие параметра 'url' в запросе.", http.StatusBadRequest)
		return
	}
	if !methodExists {
		http.Error(w, "Отсутствие параметра 'method' в запросе.", http.StatusBadRequest)
		return
	}

	var jsonMessageDecodedBytes []byte
	if messageExists {
		jsonMessageDecodedBytes, err = base64.StdEncoding.DecodeString(jsonMessageStr)
		if err != nil {
			http.Error(w, "Не удалось преобразовать тело вложенного запроса в байты.", http.StatusBadRequest)
			return
		}
	}

	// Создаём HTTP-запрос на указанный URL
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonMessageDecodedBytes))
	if err != nil {
		http.Error(w, "Ошибка создания HTTP-запроса", http.StatusInternalServerError)
		return
	}

	req.Close = true

	// Выполняем HTTP-запрос
	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		http.Error(w, "Ошибка выполнения HTTP-запроса", http.StatusInternalServerError)
		return
	}
	defer response.Body.Close()

	// Читаем тело ответа
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела ответа", http.StatusInternalServerError)
		return
	}

	// Отправляем код ответа и тело ответа клиенту
	w.WriteHeader(response.StatusCode)
	w.Write(respBody)
}

func main() {
	http.HandleFunc("/makeRequest", makeRequestHandler)
	http.ListenAndServe(":1234", nil)
}
