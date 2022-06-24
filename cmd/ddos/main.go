package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/http"
)

func CalculateLuhn(number int) int {
	checkNumber := checksum(number)

	if checkNumber == 0 {
		return 0
	}
	return 10 - checkNumber
}

func checksum(number int) int {
	var luhn int

	for i := 0; number > 0; i++ {
		cur := number % 10

		if i%2 == 0 { // even
			cur = cur * 2
			if cur > 9 {
				cur = cur%10 + cur/10
			}
		}

		luhn += cur
		number = number / 10
	}
	return luhn % 10
}

func generate() int64 {
	i := rand.Intn(1000000000)
	last := CalculateLuhn(i)
	i = i*10 + last

	return int64(i)
}

func main() {
	m := map[int64]struct{}{}
	client := http.Client{}
	login := []byte(`{"login":"gopher","password":"qwerty"}`)
	reg, err := http.NewRequest(http.MethodPost, "http://localhost:8000/api/user/register", bytes.NewBuffer(login))
	if err != nil {
		fmt.Println(err)
		return
	}
	reg.Header.Set("Content-type", "application/json")

	l, err := client.Do(reg)
	l.Body.Close()

	token := l.Header.Get("Authorization")
	fmt.Println(token, l.StatusCode)

	for i := 0; i < 1000; i++ {
		number := generate()
		_, ok := m[number]
		if ok {
			fmt.Println("duble")
		}
		m[number] = struct{}{}
		body := []byte(fmt.Sprintf("%d", number))
		req, err := http.NewRequest(http.MethodPost, "http://localhost:8000/api/user/orders", bytes.NewBuffer(body))
		req.Header.Set("Content-type", "text/plain")
		req.Header.Set("Authorization", token)
		if err != nil {
			fmt.Println(err)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}

		if resp.StatusCode != http.StatusAccepted {
			fmt.Println(resp.StatusCode)
		}
	}
}
