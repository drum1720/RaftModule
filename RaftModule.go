package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"
)

type RaftModule struct {
	port          string
	status        string
	timeout       int
	CountServices int
	servicesPorts []string
	leaderIsLive  bool
}

func (rm *RaftModule) GetPort() string {
	return rm.port
}

func NewRaftModule(minPort int, countServices int) (RaftModule, error) {
	//проверяется указанное количество портов идущих подряд,начиная с указанного MinPort,
	//если среди них нет ни одного свободного, вернется пустая структура и ошибка
	rand.Seed(time.Now().UnixNano())
	timeout := rand.Intn(100) + 100 //выбирается рандомом от 100 до 200
	port := ""
	servicesPorts := make([]string, 0, countServices)

	for i := 0; i < countServices; i++ {
		servicesPorts = append(servicesPorts, strconv.Itoa(minPort+i))
		ln, err := net.Listen("tcp", ":"+servicesPorts[i])
		if ln != nil {
			_ = ln.Close()
		}
		if err == nil && port == "" {
			port = servicesPorts[i]
		}
	}
	if port == "" {
		return RaftModule{}, errors.New("Ports are busy")
	}

	return RaftModule{
		port,
		"follower",
		timeout,
		countServices,
		servicesPorts,
		false}, nil
}

func (rm *RaftModule) Start() {
	go rm.listen()
	rm.sendMessage()
}

func (rm *RaftModule) sendMessage() {
	for {
		rm.leaderIsLive = false

		if rm.status != "leader" { //всем кроме лидера на старте дается рандомная небольшая задержка
			time.Sleep(time.Millisecond * time.Duration(rm.timeout))
		}

		time.Sleep(time.Second) //все дружно ждут одну секунду

		if rm.leaderIsLive == true { //если в это время пришло сообщение от лидера, на этом всё.
			continue
		}

		if rm.status == "leader" {
			if rm.majorityIsAvailable() {
				rm.leaderMessage()
				continue
			} else {
				rm.status = "follower"
			}
		}

		time.Sleep(time.Second * 2)
		if !rm.leaderIsLive {
			rm.voting()
		}
	}
}

func (rm *RaftModule) leaderMessage() {
	client := http.Client{}

	for _, h := range rm.servicesPorts {
		req, _ := http.NewRequest("GET", "http://localhost:"+h+"/mp", nil)
		req.Header.Set("server_status", "leader")
		req.Header.Set("port", rm.port)
		go client.Do(req)
	}
}

func (rm *RaftModule) majorityIsAvailable() bool {
	countAvailable := 0
	timeout := time.Second / 30
	client := http.Client{
		Timeout: timeout,
	}

	for _, h := range rm.servicesPorts {
		req, _ := http.NewRequest("GET", "http://localhost:"+h+"/ping", nil)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode == 200 {
			countAvailable++
		}
	}

	return rm.CountServices/2 < countAvailable
}

func (rm *RaftModule) voting() {
	rm.status = "candidate"
	countVoices := 0

	timeout := time.Second / 20
	client := http.Client{
		Timeout: timeout,
	}

	for _, h := range rm.servicesPorts {
		req, _ := http.NewRequest("GET", "http://localhost:"+h+"/mp", nil)
		req.Header.Set("server_status", "candidate")
		r, err := client.Do(req)
		if err != nil {
			continue
		}
		if r.Header.Get("voice") == "yes" {
			countVoices++
		}
	}

	if rm.CountServices/2 < countVoices && rm.status == "candidate" {
		rm.status = "leader"
	} else {
		rm.status = "follower"
	}
}

func (rm *RaftModule) listen() {
	http.HandleFunc("/ping", ping)
	http.HandleFunc("/mp", rm.messageProcessing)
	fmt.Println("listen " + rm.port)
	err := http.ListenAndServe(":"+rm.port, nil)
	if err != nil {
		panic(err)
	}

}

func ping(w http.ResponseWriter, r *http.Request) {
}

func (rm *RaftModule) messageProcessing(w http.ResponseWriter, r *http.Request) {
	status := r.Header.Get("server_status")

	switch status {
	case "leader":
		if r.Header.Get("port") == rm.port {
			fmt.Println("i'm Leader")
		} else {
			rm.status = "follower"
			fmt.Println("i'm follower")
			rm.leaderIsLive = true
		}
	case "candidate":
		if rm.status != "leader" {
			w.Header().Add("voice", "yes")
		}
		rm.leaderIsLive = true
	}
}
