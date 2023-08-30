package tangle

import (
	"blockChain_consensus/tangleChain/common"
	"blockChain_consensus/tangleChain/database"
	loglogrus "blockChain_consensus/tangleChain/log_logrus"
	"blockChain_consensus/tangleChain/message"
	"blockChain_consensus/tangleChain/p2p"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

const (
	TipExpireTime = 20 * time.Second
)

type Tangel struct {
	peer          *p2p.Peer
	Database      database.Database // 专门存储区块的数据库
	DatabaseMutex sync.RWMutex

	WorldState      database.Database // 存储世界状态的数据库
	WorldStateMutex sync.RWMutex

	L0 int           // tip节点的数量
	λ  int           // 交易的生成速率(达不到的话就用空交易)
	h  time.Duration // 交易从生成到确认的时间间隔

	tipExpireTime time.Duration // tip交易任期时长(这个时长会影响DAG分叉程度和TipSet的大小) TODO:需要更加合理的设置这个参数

	GenesisTx   *Transaction
	TangleGraph *Transaction // 以Genesis为根节点的有向图结构

	TipSet        map[common.Hash]time.Time       // 存储当前的所有Tip(key值是交易哈希值,value记录交易成为tip的时间点)
	CandidateTips map[common.Hash]*RawTransaction // 候选Tip交易

	curTipMutex       sync.RWMutex
	candidateTipMutex sync.RWMutex

	txCount      int // 已经上链的交易总数(不包含创世交易,从1开始计数)
	txCountMutex sync.RWMutex

	stopChannel chan bool
}

func NewTangle(λ int, h time.Duration, peer *p2p.Peer) *Tangel {
	tangle := &Tangel{
		peer:          peer,
		λ:             λ,
		h:             h,
		tipExpireTime: TipExpireTime,
		stopChannel:   make(chan bool),
	}
	if memDB1, err := leveldb.Open(storage.NewMemStorage(), nil); err != nil {
		loglogrus.Log.Errorf("当前节点(%s:%d)无法创建内存数据库,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port, err)
		return nil
	} else {
		tangle.Database = database.NewSimpleLDB("Transaction", memDB1)
	}

	if memDB2, err := leveldb.Open(storage.NewMemStorage(), nil); err != nil {
		loglogrus.Log.Errorf("当前节点(%s:%d)无法创建内存数据库,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port, err)
		return nil
	} else {
		tangle.WorldState = database.NewSimpleLDB("WorldState", memDB2)
	}

	// 将创始交易存入数据库
	genesis := NewGenesisTx(common.NodeID{})
	tangle.DatabaseMutex.Lock()
	key := genesis.RawTx.TxID[:]
	value := TransactionSerialize(genesis.RawTx)
	tangle.Database.Put(key, value)
	tangle.DatabaseMutex.Unlock()

	tangle.L0 = 2 * λ * int(h.Seconds())

	tangle.GenesisTx = genesis
	tangle.TangleGraph = genesis
	tangle.TipSet = make(map[common.Hash]time.Time)
	tangle.TipSet[genesis.RawTx.TxID] = time.Now()
	tangle.CandidateTips = make(map[common.Hash]*RawTransaction)

	return tangle
}

func (tg *Tangel) Start(ctx context.Context) {
	go tg.ReadMsgFromP2PPool(ctx) // 启动tangle节点的接收协程
	go tg.UpdateTipSet(ctx)       // 更新tangle节点的tip交易
}

func (tg *Tangel) ReadMsgFromP2PPool(ctx context.Context) {
	cycle := time.NewTicker(tg.h / 2)
	for {
		select {
		case <-cycle.C:
			allMsg := tg.peer.BackAllMsg()
			//fmt.Printf("[Tangle] 当前节点(%s:%d)p2p消息池中的消息数量: %d\n", tg.peer.LocalAddr.IP, tg.peer.LocalAddr.Port, len(allMsg))

			txSet := make([]*Transaction, 0)
			for _, msg := range allMsg {
				switch msg.MsgType() {
				case message.CommonCode:
					txJsonStr := msg.BackPayload().(string)
					if tx := DecodeTxFromJsonStr(txJsonStr); tx != nil {
						txSet = append(txSet, tx)
					}
					msg.MarkRetrieved()
				}
			}
			go tg.DealRcvTransaction(txSet)

		case <-ctx.Done():
			cycle.Stop()
			return
		default:
			continue
		}
	}
}

// 负责处理接收到的来自于其他节点发布的tangle交易（1.验证Pow   2.合法交易加入到CandidateTips）
func (tg *Tangel) DealRcvTransaction(txs []*Transaction) {
	validTxs := make([]*Transaction, 0) // 存储所有有效的交易(能通过Pow验证)
	for _, tx := range txs {
		if tx.PowValidator() {

			var approveTxStr string
			for index, aTx := range tx.RawTx.ApproveTx {
				approveTxStr += fmt.Sprintf("支持的第%d笔交易 (txID:%x)    ", index, aTx)
			}
			loglogrus.Log.Infof("[Tangle] 当前节点(%s:%d)(NodeID:%x)完成对来自Node(%x)交易(%x)的Pow验证, %s  \n", tg.peer.LocalAddr.IP,
				tg.peer.LocalAddr.Port, tg.peer.BackNodeID(), tx.RawTx.Sender, tx.RawTx.TxID, approveTxStr)

			validTxs = append(validTxs, tx)
		}
	}

	tg.candidateTipMutex.Lock()
	for _, validTx := range validTxs {
		tg.CandidateTips[validTx.RawTx.TxID] = validTx.RawTx
	}

	tg.candidateTipMutex.Unlock()
}

// 定期使用candidate更新tangle的tip集合(建立在一种特殊的tip策略上：一个新生成的区块只有经历固定的时间长度后才能成为tip)
func (tg *Tangel) UpdateTipSet(ctx context.Context) {
	cycle := time.NewTicker(tg.h / 2)
	tipExpireCycle := time.NewTicker(tg.tipExpireTime)

	for {
		select {
		case <-ctx.Done():
			cycle.Stop()
			tipExpireCycle.Stop()
			return
		case <-tipExpireCycle.C:
			now := time.Now()
			tg.curTipMutex.Lock()

			for tip, _ := range tg.TipSet {
				loglogrus.Log.Infof("[Tangle] 当前节点(%s:%d) 更新前的(tipCount:%d) tip(%x)\n", tg.peer.LocalAddr.IP, tg.peer.LocalAddr.Port, len(tg.TipSet), tip)
			}

			for txID, eleTime := range tg.TipSet {
				if now.Sub(eleTime) > tg.tipExpireTime {
					delete(tg.TipSet, txID)
				}
			}
			tg.curTipMutex.Unlock()

			for tip, _ := range tg.TipSet {
				loglogrus.Log.Infof("[Tangle] 当前节点(%s:%d) 更新后的(tipCount:%d) tip(%x)\n", tg.peer.LocalAddr.IP, tg.peer.LocalAddr.Port, len(tg.TipSet), tip)
			}

		case <-cycle.C:

			if len(tg.TipSet) >= tg.L0 { // tip节点的数量不能超过L0
				continue
			}

			now := time.Now().UnixNano()
			tg.curTipMutex.Lock()
			tg.candidateTipMutex.Lock()
			for _, candidate := range tg.CandidateTips {
				if uint64(now)-candidate.TimeStamp > uint64(tg.h.Nanoseconds()) { // 交易可以被确认(也即是可以真正上链)

					loglogrus.Log.Infof("[Tangle] 当前节点(%s:%d) 的 candidate 交易(%x)可以进行上链,变为 Tip 交易, len(PreviousTxs) = %d\n",
						tg.peer.LocalAddr.IP, tg.peer.LocalAddr.Port, candidate.TxID, len(candidate.PreviousTxs))

					tg.TipSet[candidate.TxID] = time.Now()

					tg.DatabaseMutex.Lock()
					key := candidate.TxID[:]
					value := TransactionSerialize(candidate)
					tg.Database.Put(key, value)
					tg.DatabaseMutex.Unlock()
					delete(tg.CandidateTips, candidate.TxID) // 将此上链的交易从候选tip集合中删除

					// 更新当前交易的approveTx的相关信息(其实只需要更新其中的tip部分即可)
					candidateTx := &Transaction{RawTx: candidate}
					tg.DatabaseMutex.Lock()
					candidateTx.UpdatePreviousTx(tg.Database)
					tg.DatabaseMutex.Unlock()

					// 执行 candidateTx 交易
					tg.WorldStateMutex.Lock()

					switch candidateTx.RawTx.TxCode {
					case CommonWriteCode:
						candidateTx.CommonExecuteWrite(tg.WorldState, "test-key", "test-value")
					case CommonReadCode:
						res := candidateTx.CommonExecuteRead(tg.WorldState, "test-key")
						loglogrus.Log.Infof("[Tangle] 当前节点(%s:%d) 的 candidate(%x) 交易执行结果为: %s",
							tg.peer.LocalAddr.IP, tg.peer.LocalAddr.Port, candidate.TxID, res)
					case CommonWriteAndReadCode:
						candidateTx.CommonExecuteWrite(tg.WorldState, "test-key", "test-value")
						res := candidateTx.CommonExecuteRead(tg.WorldState, "test-key")
						loglogrus.Log.Infof("[Tangle] 当前节点(%s:%d) 的 candidate(%x) 交易执行结果为: %s",
							tg.peer.LocalAddr.IP, tg.peer.LocalAddr.Port, candidate.TxID, res)
					}

					tg.WorldStateMutex.Unlock()

					tg.txCountMutex.Lock()
					tg.txCount++ // 上链交易数+1
					tg.txCountMutex.Unlock()
				}
			}
			tg.curTipMutex.Unlock()
			tg.candidateTipMutex.Unlock()
		}
	}

}

func (tg *Tangel) BackTxCount() int {
	tg.txCountMutex.RLock()
	defer tg.txCountMutex.RUnlock()

	return tg.txCount
}

// 发布一笔交易
func (tg *Tangel) PublishTransaction(data interface{}, txCode uint64) {
	tg.curTipMutex.RLock()
	tipSet := make([]common.Hash, 0)
	for tip, _ := range tg.TipSet {
		tipSet = append(tipSet, tip)
	}
	tg.curTipMutex.RUnlock()

	// for tip, _ := range tg.TipSet {
	// 	loglogrus.Log.Infof("[Tangle] 当前节点(%s:%d)即将发布的交易的Previous Tx(txCount:%d) (txID:%x)\n", tg.peer.LocalAddr.IP, tg.peer.LocalAddr.Port, len(tg.TipSet), tip)
	// }

	newTx := NewTransaction(data, tipSet, tg.peer.BackNodeID(), txCode)

	tg.DatabaseMutex.Lock()
	newTx.SelectApproveTx(tg.Database)
	tg.DatabaseMutex.Unlock()

	// for index, aTx := range newTx.RawTx.ApproveTx {
	// 	loglogrus.Log.Infof("[Tangle] 当前节点(%s:%d)即将发布交易支持的第(%d)笔交易是 (TxID:%x)\n", tg.peer.LocalAddr.IP, tg.peer.LocalAddr.Port, index, aTx)
	// }

	newTx.Pow()
	loglogrus.Log.Infof("[Tangle] 当前节点(%s:%d) Pow计算得到的新交易的 TxID(%x) 此时的Nonce(%d)\n", tg.peer.LocalAddr.IP, tg.peer.LocalAddr.Port, newTx.RawTx.TxID, newTx.RawTx.Nonce)

	// 需要将该交易广播出去
	wrapMsg := EncodeTxToWrapMsg(newTx, tg.peer.BackPrvKey())
	tg.peer.Broadcast(wrapMsg)

	tg.candidateTipMutex.Lock()
	tg.CandidateTips[newTx.RawTx.TxID] = newTx.RawTx // 交易加入到候选tip集合
	tg.candidateTipMutex.Unlock()
}
