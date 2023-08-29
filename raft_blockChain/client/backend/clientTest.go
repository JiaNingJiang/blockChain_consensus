package main

import (
	"fmt"
	"raftClient/client"
	"time"
)

func main() {

	raftNodes := []string{"http://127.0.0.1:7001/newTx", "http://127.0.0.1:8001/newTx", "http://127.0.0.1:9001/newTx"}

	client := client.NewClient("client1", raftNodes)

	for i := 0; i < 10; i++ {
		time.Sleep(5 * time.Nanosecond)
		client.CommonWrite([]string{fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i)})
	}

	for {
		time.Sleep(time.Second)
	}
}
