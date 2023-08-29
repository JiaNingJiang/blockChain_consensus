package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"raftClient/api/httpApi/app"
	errorMsg "raftClient/api/httpApi/error"
	"raftClient/contract"
	"raftClient/fsm"
	loglogrus "raftClient/log_logrus"

	"github.com/astaxie/beego/validation"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/raft"
)

const (
	minWaitingTxCount  = 20
	maxWaitingDuration = 3 * time.Second
)

type RecordInfo struct {
	txPool               map[string]struct{} // 累计从客户端搜集的交易
	txPoolLastHandleTime time.Time           // 上一次处理交易池中交易的时间
	txPoolMutex          sync.RWMutex
}

func InitRouter(raft *raft.Raft, fsm *fsm.Fsm, addrMap map[string]string) *gin.Engine {

	recordInfo := &RecordInfo{
		txPool:               make(map[string]struct{}),
		txPoolLastHandleTime: time.Now(),
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	gin.SetMode("release")

	r.GET("/set", CommonSet(raft, fsm)) // 简单的set操作(key-value)
	r.GET("/get", CommonGet(raft, fsm)) // 简单的get操作

	r.GET("/leader", GetRaftLeader(raft, fsm, addrMap))
	r.POST("/newTx", CreateNewTransaction(raft, fsm)) // 根据请求的内容创建一笔交易
	r.POST("/newTx_highTPS", CreateTransactionSet(raft, fsm, recordInfo))

	return r
}

// 单次写入
func CommonSet(raft *raft.Raft, fsm *fsm.Fsm) func(*gin.Context) {
	return func(c *gin.Context) {
		appG := app.Gin{c}
		pair := c.Request.URL.Query()
		key := pair.Get("key")
		value := pair.Get("value")
		if key == "" || value == "" {
			appG.Response(http.StatusOK, errorMsg.ERROR, "error key or value")
			return
		}

		data := "set" + "-" + key + "-" + value
		future := raft.Apply([]byte(data), 5*time.Second) // 使用fsm.Apply()完成数据的写入(因为写操作会改变数据库状态)
		if err := future.Error(); err != nil {
			appG.Response(http.StatusOK, errorMsg.ERROR, err.Error())
			return
		}
		appG.Response(http.StatusOK, errorMsg.SUCCESS, "set successfully")
	}
}

// 单次读取
func CommonGet(raft *raft.Raft, fsm *fsm.Fsm) func(*gin.Context) {
	return func(c *gin.Context) {
		appG := app.Gin{c}
		pair := c.Request.URL.Query()
		key := pair.Get("key")
		if key == "" {
			appG.Response(http.StatusOK, errorMsg.ERROR, "error key")
			return
		}
		value := fsm.DataBase.Get(key)
		appG.Response(http.StatusOK, errorMsg.SUCCESS, value)
	}
}

func GetRaftLeader(raft *raft.Raft, fsm *fsm.Fsm, addrMap map[string]string) func(*gin.Context) {
	return func(c *gin.Context) {
		appG := app.Gin{c}
		leaderAddr := raft.Leader()

		leaderRaftAddr := fmt.Sprintf("%v", leaderAddr)

		appG.Response(http.StatusOK, errorMsg.SUCCESS, fmt.Sprintf("http://%v", addrMap[leaderRaftAddr]))

	}
}

// 生成交易后立即执行,即一轮共识执行一笔交易(TODO:当前无论读写交易都视为raft写操作)
func CreateNewTransaction(raft *raft.Raft, fsm *fsm.Fsm) func(*gin.Context) {
	return func(c *gin.Context) {
		appG := app.Gin{c}
		contractName := c.PostForm("contractName") // 合约名
		functionName := c.PostForm("functionName") // 合约函数名
		args := c.PostForm("args")                 // 函数参数
		valid := validation.Validation{}
		valid.Required(contractName, "contractName").Message("合约名不能为空")
		valid.Required(functionName, "functionName").Message("函数名不能为空")
		valid.Required(args, "args").Message("参数不能为空")

		if valid.HasErrors() {
			app.MarkErrors("CreateNewTransaction", valid.Errors)
			appG.Response(http.StatusOK, errorMsg.INVALID_PARAMS, nil)
			return
		}

		tx := "tx" + "-" + contractName + "-" + functionName + "-" + args
		tx += "-" + fmt.Sprintf("%d", time.Now().UnixNano()) // 为了区分就有相同内容但不同时间到达的交易,需要一个时间戳将这些交易区分开(精确到纳秒)

		// TODO:如何为了提高读取的TPS，可以在此处检测所有"读"的合约函数,在本地完成读取操作,然后返回
		switch functionName {
		case "Read":
			argSet := strings.Split(args, " ")
			if res, err := contract.ContractFuncRun(fsm.DataBase, contractName, functionName, argSet); err != nil {
				loglogrus.Log.Warnf("[FSM] 合约执行错误,err:%v\n", err)
				appG.Response(http.StatusOK, errorMsg.ERROR, err.Error())
				return
			} else {
				appG.Response(http.StatusOK, errorMsg.SUCCESS, res)
				return
			}

		}

		// 对于写操作,执行下面的代码
		future := raft.Apply([]byte(tx), 5*time.Second) // 使用fsm.Apply()完成交易的执行(后面的时间是等待raft执行的时间，若超时相当于本次共识失败,返回超时作为错误原因)
		if err := future.Error(); err != nil {
			appG.Response(http.StatusOK, errorMsg.ERROR, err.Error())
			return
		}

		appG.Response(http.StatusOK, errorMsg.SUCCESS, future.Response())

	}
}

// 针对TPS要求较高的环境,集中式处理一批获取到的交易(该方法只对写入操作的TPS有改善,对读取TPS没有太大提升)
func CreateTransactionSet(raft *raft.Raft, fsm *fsm.Fsm, recordInfo *RecordInfo) func(*gin.Context) {
	return func(c *gin.Context) {
		appG := app.Gin{c}
		contractName := c.PostForm("contractName") // 合约名
		functionName := c.PostForm("functionName") // 合约函数名
		args := c.PostForm("args")                 // 函数参数
		valid := validation.Validation{}
		valid.Required(contractName, "contractName").Message("合约名不能为空")
		valid.Required(functionName, "functionName").Message("函数名不能为空")
		valid.Required(args, "args").Message("参数不能为空")

		if valid.HasErrors() {
			app.MarkErrors("CreateNewTransaction", valid.Errors)
			appG.Response(http.StatusOK, errorMsg.INVALID_PARAMS, nil)
			return
		}

		tx := "tx" + "-" + contractName + "-" + functionName + "-" + args
		tx += "-" + fmt.Sprintf("%d", time.Now().UnixNano()) // 为了区分就有相同内容但不同时间到达的交易,需要一个时间戳将这些交易区分开

		loglogrus.Log.Infof("[Http Interface] 接收到客户端交易请求:%s\n", tx)

		// 为了提高整体的TPS,可以在获取这一笔交易后,将其添加到等待池,凑够足够数量的交易后再统一处理
		recordInfo.txPoolMutex.Lock()
		recordInfo.txPool[tx] = struct{}{}
		// appG.Response(http.StatusOK, errorMsg.SUCCESS, "")
		recordInfo.txPoolMutex.Unlock()

		// || time.Now().Sub(recordInfo.txPoolLastHandleTime) >= maxWaitingDuration
		if len(recordInfo.txPool) >= minWaitingTxCount {

			loglogrus.Log.Infof("[Http Interface] 当前交易池中已经收集到的交易数量:%d\n", len(recordInfo.txPool))

			recordInfo.txPoolMutex.Lock()
			txSet := make([]string, 0)
			for tx, _ := range recordInfo.txPool {
				txSet = append(txSet, tx)
			}

			txResults := make([]string, 0) // 存储对应交易的执行结果
			for _, tx := range txSet {
				delete(recordInfo.txPool, tx)
				loglogrus.Log.Infof("[Http Interface] 准备执行交易:%s\n", tx)
				future := raft.Apply([]byte(tx), 3*time.Second) // 使用fsm.Apply()完成交易的执行(后面的时间是等待raft执行的时间，若超时相当于本次共识失败,返回超时作为错误原因)
				if err := future.Error(); err != nil {
					txResults = append(txResults, fmt.Sprintf("%v", err.Error()))
				} else {
					txResults = append(txResults, fmt.Sprintf("%v", future.Response()))
				}
			}
			recordInfo.txPoolMutex.Unlock()
			appG.Response(http.StatusOK, errorMsg.SUCCESS, txResults)
		}

	}
}
