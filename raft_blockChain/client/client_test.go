package client

import (
	"fmt"
	"testing"
	"time"
)

func TestGetLeader(t *testing.T) {
	raftNodes := []string{"http://127.0.0.1:7001", "http://127.0.0.1:8001", "http://127.0.0.1:9001"}

	client := NewClient("client1", raftNodes)

	fmt.Println(client.GetLeaderHttp())
}

func TestClientWrite(t *testing.T) {
	raftNodes := []string{"http://127.0.0.1:7001", "http://127.0.0.1:8001", "http://127.0.0.1:9001"}

	client := NewClient("client1", raftNodes)

	beforeTime := time.Now()
	for i := 0; i < 1000; i++ {
		// time.Sleep(2 * time.Nanosecond)
		res := client.CommonWrite([]string{fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i)})

		fmt.Printf("本轮(%d)结果 : %s\n", i, res)
	}
	afterTime := time.Now()

	fmt.Printf("读取之前时间:%s --- 读取之后时间:%s --- 时间差:%v \n", beforeTime.Format("2006-01-02 15:04:05"), afterTime.Format("2006-01-02 15:04:05"), afterTime.Sub(beforeTime).Seconds())

}

func TestClientRead(t *testing.T) {
	raftNodes := []string{"http://127.0.0.1:7001", "http://127.0.0.1:8001", "http://127.0.0.1:9001"}

	client := NewClient("client1", raftNodes)

	res := client.CommonWrite([]string{"111", "222"})
	fmt.Printf("写入结果 : %s\n", res)

	beforeTime := time.Now()
	for i := 0; i < 10000; i++ {
		res := client.CommonRead([]string{"111"})
		fmt.Printf("本轮(%d)结果 : %s\n", i, res)
	}
	afterTime := time.Now()

	fmt.Printf("读取之前时间:%s --- 读取之后时间:%s --- 时间差:%v \n", beforeTime.Format("2006-01-02 15:04:05"), afterTime.Format("2006-01-02 15:04:05"), afterTime.Sub(beforeTime).Seconds())

}
