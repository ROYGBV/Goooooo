package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"sync"
)

// Структура тела запроса
type Request struct {
	Url    string `json:"url"`
	Method string `json:"method"`
	Body   string `json:"body"`
}

// Структура списка запросов
type RequestList struct {
	Requests []Request `json:"requests"`
}

// Структура содержимого канала, содержит порядковый номер запроса в исходном post запросе и ответ на запрос
type Result struct {
	Index    int
	Response string
}

// Выполнение одного запроса
func makeSingleRequest(index int, req Request, wg *sync.WaitGroup, ch chan Result) {
	defer wg.Done()

	res := new(Result)
	res.Index = index

	// Декодирование сообщения при его наличии
	var jsonMessageDecodedBytes []byte
	var err error
	if req.Body != "" {
		jsonMessageDecodedBytes, err = base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			res.Response = "Некорректный ввод тела вложенного запроса: " + string(err.Error()) + "\n\n"
			ch <- *res
			return
		}
	}

	// Проверяем наличие url и method
	if req.Url == "" {
		res.Response = "Отсутствие параметра 'url' в запросе." + "\n\n"
		ch <- *res
		return
	}
	if req.Method == "" {
		res.Response = "Отсутствие параметра 'method' в запросе" + "\n\n"
		ch <- *res
		return
	}

	// Создаём HTTP-запрос на указанный URL
	var reqToSend *http.Request
	reqToSend, err = http.NewRequest(req.Method, req.Url, bytes.NewBuffer(jsonMessageDecodedBytes))
	if err != nil {
		res.Response = "Ошибка создания HTTP-запроса: " + string(err.Error()) + "\n\n"
		ch <- *res
		return
	}

	reqToSend.Close = true

	// Выполняем HTTP-запрос
	client := &http.Client{}
	response, err := client.Do(reqToSend)
	if err != nil {
		res.Response = "Ошибка выполнения HTTP-запроса: " + string(err.Error()) + "\n\n"
		ch <- *res
		return
	}
	defer response.Body.Close()

	// Читаем тело ответа
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		res.Response = "Ошибка чтения тела ответа:" + string(err.Error()) + "\n\n"
		ch <- *res
		return
	}

	// Добавляем в канал код и тело ответа
	res.Response = "Код ответа: " + strconv.Itoa(response.StatusCode) + "\nТело ответа:\n" + string(respBody) + "\n\n"
	ch <- *res
}

func makeMultipleRequests(w http.ResponseWriter, r *http.Request) {
	// Проверяем, отправлен ли именно POST запрос от клиента
	if r.Method != http.MethodPost {
		http.Error(w, "Поддерживаются только POST запросы", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем тело запроса на возможность считать его
	rBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса: "+string(err.Error()), http.StatusBadRequest)
		return
	}

	// Извлекаем параметры из тела запроса
	var decodedRequestList RequestList
	err = json.Unmarshal(rBody, &decodedRequestList)
	if err != nil {
		http.Error(w, "Ошибка извлечения параметров из тела запроса: "+string(err.Error()), http.StatusBadRequest)
		return
	}

	// Создаём WaitGroup для ожидания завершения выполнения всех запросов и канал
	var wg sync.WaitGroup
	wg.Add(len(decodedRequestList.Requests))
	ch := make(chan Result, len(decodedRequestList.Requests))

	// Выполняем все запросы
	for i, req := range decodedRequestList.Requests {
		go makeSingleRequest(i, req, &wg, ch)
	}
	wg.Wait()
	close(ch)

	// Заносим все результаты запросов в массив для сортировки и вывода
	var resArr []Result
	for res := range ch {
		resArr = append(resArr, res)
	}
	sort.Slice(resArr[:], func(i, j int) bool {
		return resArr[i].Index < resArr[j].Index
	})
	for _, res := range resArr {
		fmt.Fprintf(w, res.Response)
	}
}

func main() {
	http.HandleFunc("/makeRequest", makeMultipleRequests)
	http.ListenAndServe(":1234", nil)
}
