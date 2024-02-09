package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sort"
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

// Структура содержимого канала, содержит порядковый номер запроса в исходном post запросе, код ответа, тело ответа, ошибку обработки
type Response struct {
	Index        int    `json:"index"`
	ResponseCode int    `json:"responsecode"`
	Response     string `json:"response"`
	Error        string `json:"error"`
}

// Структура списка ответов для формирования полного ответа
type ResponseList struct {
	Responses []Response `json:"responses"`
}

// Проверка корректности запроса
func validateRequest(w http.ResponseWriter, r *http.Request) (bool, []byte) {
	var rBody []byte
	var err error

	// Проверяем, отправлен ли именно POST запрос от клиента
	if r.Method != http.MethodPost {
		http.Error(w, "Поддерживаются только POST запросы", http.StatusMethodNotAllowed)
		return false, rBody
	}

	// Проверяем тело запроса на возможность считать его
	rBody, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса: "+string(err.Error()), http.StatusBadRequest)
		return false, rBody
	}

	// Проверка на пустой запрос
	if len(rBody) == 0 {
		http.Error(w, "Получение пустого запроса.", http.StatusBadRequest)
		return false, rBody
	}

	return true, rBody
}

// Выполнение одного запроса
func makeSingleRequest(index int, req Request, wg *sync.WaitGroup, ch chan Response) {
	defer wg.Done()

	res := new(Response)
	res.Index = index

	// Декодирование сообщения при его наличии
	var jsonMessageDecodedBytes []byte
	var err error
	if req.Body != "" {
		jsonMessageDecodedBytes, err = base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			res.Error = "Некорректный ввод тела вложенного запроса: " + string(err.Error())
			ch <- *res
			return
		}
	}

	// Проверяем наличие url и method
	if req.Url == "" {
		res.Error = "Отсутствие параметра 'url' в запросе."
		ch <- *res
		return
	}
	if req.Method == "" {
		res.Error = "Отсутствие параметра 'method' в запросе"
		ch <- *res
		return
	}

	// Создаём HTTP-запрос на указанный URL
	var reqToSend *http.Request
	reqToSend, err = http.NewRequest(req.Method, req.Url, bytes.NewBuffer(jsonMessageDecodedBytes))
	if err != nil {
		res.Error = "Ошибка создания HTTP-запроса: " + string(err.Error())
		ch <- *res
		return
	}

	// reqToSend.Close = true

	// Выполняем HTTP-запрос
	client := &http.Client{}
	response, err := client.Do(reqToSend)
	if err != nil {
		res.Error = "Ошибка выполнения HTTP-запроса: " + string(err.Error())
		ch <- *res
		return
	}
	defer response.Body.Close()

	// Читаем тело ответа
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		res.Error = "Ошибка чтения тела ответа:" + string(err.Error())
		ch <- *res
		return
	}

	// Добавляем в канал код и тело ответа
	res.ResponseCode = response.StatusCode
	res.Response = string(respBody)
	ch <- *res
}

// Обработчик множества запросов
func makeMultipleRequestsHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем корректность ввода запроса
	noError, rBody := validateRequest(w, r)
	if !noError {
		return
	}

	// Извлекаем параметры из тела запроса
	var decodedRequestList RequestList
	err := json.Unmarshal(rBody, &decodedRequestList)
	if err != nil {
		http.Error(w, "Ошибка извлечения параметров из тела запроса: "+string(err.Error()), http.StatusBadRequest)
		return
	}

	// Создаём WaitGroup для ожидания завершения выполнения всех запросов и канал
	var wg sync.WaitGroup
	wg.Add(len(decodedRequestList.Requests))
	ch := make(chan Response, len(decodedRequestList.Requests))

	// Выполняем все запросы
	for i, req := range decodedRequestList.Requests {
		go makeSingleRequest(i, req, &wg, ch)
	}
	wg.Wait()
	close(ch)

	// Заносим все результаты запросов в массив для сортировки
	var resArr ResponseList
	for res := range ch {
		resArr.Responses = append(resArr.Responses, res)
	}
	sort.Slice(resArr.Responses[:], func(i, j int) bool {
		return resArr.Responses[i].Index < resArr.Responses[j].Index
	})

	// Создаём вывод
	makeOutput(w, resArr)
}

// Обработчик одного запроса
func makeSingleRequestHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем корректность ввода запроса
	noError, rBody := validateRequest(w, r)
	if !noError {
		return
	}

	// Извлекаем параметры из тела запроса
	var decodedData Request
	err := json.Unmarshal(rBody, &decodedData)
	if err != nil {
		http.Error(w, "Ошибка извлечения параметров из тела запроса: "+string(err.Error()), http.StatusBadRequest)
		return
	}

	// Создаём WaitGroup для ожидания завершения выполнения запроса и канал
	var wg sync.WaitGroup
	wg.Add(1)
	ch := make(chan Response, 1)

	// Выполняем запрос
	go makeSingleRequest(1, decodedData, &wg, ch)
	wg.Wait()
	close(ch)

	res := <-ch
	var resArr ResponseList
	resArr.Responses = append(resArr.Responses, res)

	// Создаём вывод
	makeOutput(w, resArr)
}

func makeOutput(w http.ResponseWriter, resArr ResponseList) {
	// Преобразуем структуру в JSON
	jsonResponse, err := json.Marshal(resArr)
	if err != nil {
		http.Error(w, "Ошибка преобразования в JSON: "+string(err.Error()), http.StatusInternalServerError)
		return
	}

	// Устанавливаем заголовок Content-Type
	w.Header().Set("Content-Type", "application/json")

	// Отправляем JSON-ответ клиенту
	w.Write(jsonResponse)
}

func main() {
	http.HandleFunc("/makeRequest", makeSingleRequestHandler)
	http.HandleFunc("/makeRequests", makeMultipleRequestsHandler)
	http.ListenAndServe(":1234", nil)
}
