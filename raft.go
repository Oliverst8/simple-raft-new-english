package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/rpc"
	"strings"
	"time"
)

type node struct {
	connect bool
	address string
}

// Create new node
func newNode(address string) *node {
	node := &node{}
	node.address = address
	return node
}

// State def
type State int

// status of node
const (
	Follower State = iota + 1
	Candidate
	Leader
)

// LogEntry struct
type LogEntry struct {
	LogTerm  int
	LogIndex int
	LogCMD   interface{}
}

// Raft Node
type Raft struct {
	me int

	nodes map[int]*node

	state       State
	currentTerm int
	votedFor    int
	voteCount   int

	// Collection of log entries
	log []LogEntry

	// Maximum index submitted
	commitIndex int
	// The maximum index to be applied to the state machine
	lastApplied int

	// Save the index of the next entry that needs to be sent to each node
	nextIndex []int
	// Save the highest index of the log that has been copied to each node
	matchIndex []int

	// channels
	heartbeatC chan bool
	toLeaderC  chan bool
}

// RequestVote rpc method
func (rf *Raft) RequestVote(args VoteArgs, reply *VoteReply) error {

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return nil
	}

	if rf.votedFor == -1 {
		rf.currentTerm = args.Term
		rf.votedFor = args.CandidateID
		reply.Term = rf.currentTerm
		reply.VoteGranted = true
	}

	return nil
}

// Heartbeat rpc method
func (rf *Raft) Heartbeat(args HeartbeatArgs, reply *HeartbeatReply) error {

	//If the leader node is smaller than the current node term
	if args.Term < rf.currentTerm {
		reply.Success = false
		reply.Term = rf.currentTerm
		return nil
	}

	// if only heartbeat
	rf.heartbeatC <- true
	if len(args.Entries) == 0 {
		reply.Success = true
		reply.Term = rf.currentTerm
		return nil
	}

	// If there are entries
	// The LogIndex maintained by the leader is greater than the current Follower's LogIndex
	// It means that the current Follower has lost contact, so the Follower needs to inform the Leader that it is currently
	// The maximum index so that the next heartbeat Leader returns
	if args.PrevLogIndex > rf.getLastIndex() {
		reply.Success = false
		reply.Term = rf.currentTerm
		reply.NextIndex = rf.getLastIndex() + 1
		return nil
	}

	rf.log = append(rf.log, args.Entries...)
	rf.commitIndex = rf.getLastIndex()
	reply.Success = true
	reply.Term = rf.currentTerm
	reply.NextIndex = rf.getLastIndex() + 1

	return nil
}

func (rf *Raft) rpc(port string) {
	rpc.Register(rf)
	rpc.HandleHTTP()
	go func() {
		err := http.ListenAndServe(port, nil)
		if err != nil {
			log.Fatal("listen error: ", err)
		}
	}()
}

func (rf *Raft) start() {
	rf.state = Follower
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.heartbeatC = make(chan bool)
	rf.toLeaderC = make(chan bool)

	go func() {

		rand.Seed(time.Now().UnixNano())

		for {
			switch rf.state {
			case Follower:
				select {
				case <-rf.heartbeatC:
					log.Printf("follower-%d recived heartbeat\n", rf.me)
				case <-time.After(time.Duration(rand.Intn(500-300)+300) * time.Millisecond):
					log.Printf("follower-%d timeout\n", rf.me)
					rf.state = Candidate
				}
			case Candidate:
				fmt.Printf("Node: %d, I'm candidate\n", rf.me)
				rf.currentTerm++
				rf.votedFor = rf.me
				rf.voteCount = 1
				go rf.broadcastRequestVote()

				select {
				case <-time.After(time.Duration(rand.Intn(500-300)+300) * time.Millisecond):
					rf.state = Follower
				case <-rf.toLeaderC:
					fmt.Printf("Node: %d, I'm leader\n", rf.me)
					rf.state = Leader

					// Initialize nextIndex and matchIndex of peers
					rf.nextIndex = make([]int, len(rf.nodes))
					rf.matchIndex = make([]int, len(rf.nodes))
					for i := range rf.nodes {
						rf.nextIndex[i] = 1
						rf.matchIndex[i] = 0
					}

					go func() {
						i := 0
						for {
							i++
							rf.log = append(rf.log, LogEntry{rf.currentTerm, i, fmt.Sprintf("user send : %d", i)})
							time.Sleep(3 * time.Second)
						}
					}()
				}
			case Leader:
				rf.broadcastHeartbeat()
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

type VoteArgs struct {
	Term        int
	CandidateID int
}

type VoteReply struct {
	//The current term number, so that candidates can update their term number
	Term int
	//True if the candidate wins this vote
	VoteGranted bool
}

func (rf *Raft) broadcastRequestVote() {
	var args = VoteArgs{
		Term:        rf.currentTerm,
		CandidateID: rf.me,
	}

	for i := range rf.nodes {
		go func(i int) {
			var reply VoteReply
			rf.sendRequestVote(i, args, &reply)
		}(i)
	}
}

func (rf *Raft) sendRequestVote(serverID int, args VoteArgs, reply *VoteReply) {
	client, err := rpc.DialHTTP("tcp", rf.nodes[serverID].address)
	if err != nil {
		log.Fatal("dialing: ", err)
	}

	defer client.Close()
	client.Call("Raft.RequestVote", args, reply)

	// The current candidate node is invalid
	if reply.Term > rf.currentTerm {
		rf.currentTerm = reply.Term
		rf.state = Follower
		rf.votedFor = -1
		return
	}

	if reply.VoteGranted {
		rf.voteCount++
	}

	if rf.voteCount >= len(rf.nodes)/2+1 {
		rf.toLeaderC <- true
	}
}

type HeartbeatArgs struct {
	Term     int
	LeaderID int

	// Index before new log
	PrevLogIndex int
	// The term number of PrevLogIndex
	PrevLogTerm int
	// Log entries to prepare for storage (null when indicating a heartbeat)
	Entries []LogEntry
	// Index value that Leader has committed
	LeaderCommit int
}

type HeartbeatReply struct {
	Success bool
	Term    int

	// If the Follower Index is smaller than the Leader Index, the Leader will be told the index position to start sending next time.
	NextIndex int
}

func (rf *Raft) broadcastHeartbeat() {
	for i := range rf.nodes {

		var args HeartbeatArgs
		args.Term = rf.currentTerm
		args.LeaderID = rf.me
		args.LeaderCommit = rf.commitIndex

		// Calculate preLogIndex and preLogTerm
		// Extract entries after preLogIndex - baseIndex and send them to the follower
		prevLogIndex := rf.nextIndex[i] - 1
		if rf.getLastIndex() > prevLogIndex {
			args.PrevLogIndex = prevLogIndex
			args.PrevLogTerm = rf.log[prevLogIndex].LogTerm
			args.Entries = rf.log[prevLogIndex:]
			log.Printf("send entries: %v\n", args.Entries)
		}

		go func(i int, args HeartbeatArgs) {
			var reply HeartbeatReply
			rf.sendHeartbeat(i, args, &reply)
		}(i, args)
	}
}

func (rf *Raft) sendHeartbeat(serverID int, args HeartbeatArgs, reply *HeartbeatReply) {
	client, err := rpc.DialHTTP("tcp", rf.nodes[serverID].address)
	if err != nil {
		log.Fatal("dialing:", err)
	}

	defer client.Close()
	client.Call("Raft.Heartbeat", args, reply)

	// If the leader node lags behind the follower node
	if reply.Success {
		if reply.NextIndex > 0 {
			rf.nextIndex[serverID] = reply.NextIndex
			rf.matchIndex[serverID] = rf.nextIndex[serverID] - 1
		}
	} else {
		// If the leader's term is less than the follower's term,
		// the leader must convert to a follower and re-initiate an election
		if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.state = Follower
			rf.votedFor = -1
			return
		}
	}
}

func (rf *Raft) getLastIndex() int {
	rlen := len(rf.log)
	if rlen == 0 {
		return 0
	}
	return rf.log[rlen-1].LogIndex
}

func (rf *Raft) getLastTerm() int {
	rlen := len(rf.log)
	if rlen == 0 {
		return 0
	}
	return rf.log[rlen-1].LogTerm
}

func main() {
	port := flag.String("port", ":9091", "rpc listen port")
	cluster := flag.String("cluster", "127.0.0.1:9091", "comma sep")
	id := flag.Int("id", 1, "node ID")

	flag.Parse()
	clusters := strings.Split(*cluster, ",")

	ns := make(map[int]*node)
	for k, v := range clusters {
		ns[k] = newNode(v)
	}

	raft := &Raft{}
	raft.me = *id
	raft.nodes = ns
	raft.rpc(*port)
	raft.start()

	select {}

}
