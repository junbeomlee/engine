/*
 * Copyright 2018 It-chain
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api_test

import (
	"testing"

	"strconv"

	"time"

	"github.com/it-chain/engine/consensus/pbft"
	"github.com/it-chain/engine/consensus/pbft/api"
	"github.com/it-chain/engine/consensus/pbft/infra/mem"
	test2 "github.com/it-chain/engine/consensus/pbft/test"
	"github.com/it-chain/engine/consensus/pbft/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/it-chain/iLogger"
)

var normalBlock = pbft.ProposedBlock{
	Seal: []byte{1, 2, 3, 4},
	Body: []byte{1, 2, 3, 5},
}

var errorBlock = pbft.ProposedBlock{
	Seal: nil,
	Body: nil,
}

func TestConsensusApi_StartConsensus(t *testing.T) {

	tests := map[string]struct {
		input struct {
			block      pbft.ProposedBlock
			peerNum    int
			isRepoFull bool
		}
		err error
	}{
		"Case1 : Consensus가 필요하고 Proposed된 Block이 정상이며, Repo가 차있지 않는 경우 (Normal Case)": {
			input: struct {
				block      pbft.ProposedBlock
				peerNum    int
				isRepoFull bool
			}{normalBlock, 5, false},
			err: nil,
		},
		"Case2 : Consensus가 필요없는 상황": {
			input: struct {
				block      pbft.ProposedBlock
				peerNum    int
				isRepoFull bool
			}{normalBlock, 1, false},
			err: api.ConsensusCreateError,
		},
		"Case3 : Consensus가 필요하고 Proposed된 Block이 정상이며, Repo가 차있는 경우": {
			input: struct {
				block      pbft.ProposedBlock
				peerNum    int
				isRepoFull bool
			}{normalBlock, 5, true},
			err: pbft.ErrInvalidSave,
		},
	}

	for testName, test := range tests {
		t.Logf("running test case %s ", testName)
		cApi := setUpApiCondition(test.input.peerNum, test.input.isRepoFull, true)
		assert.EqualValues(t, test.err, cApi.StartConsensus(test.input.block))

	}
}

func TestStateApiImpl_StartConsensus(t *testing.T) {
	tests := map[string]struct {
		input struct {
			processList []string
		}
	}{
		"8 node test": {
			input: struct{ processList []string }{
				processList: []string{"1", "2", "3", "4"},
			},
		},
	}

	for testName, test := range tests {
		t.Logf("running test case %s", testName)

		env := test2.SetTestEnvironment(test.input.processList)

		for _, p := range env.ProcessMap {
			process := *p
			pRepo := process.Services["ParliamentRepository"].(*mem.ParliamentRepository)
			p := pRepo.Load()
			p.SetLeader("1")
			pRepo.Save(p)
		}
		stateApi1 := env.ProcessMap["1"].Services["StateApiImpl"].(*api.StateApiImpl)
		stateRepo1 := env.ProcessMap["1"].Services["StateRepository"].(*mem.StateRepository)
		state1, _ := stateRepo1.Load()
		stateRepo2 := env.ProcessMap["2"].Services["StateRepository"].(*mem.StateRepository)
		state2, _ := stateRepo2.Load()

		stateRepo3 := env.ProcessMap["3"].Services["StateRepository"].(*mem.StateRepository)
		state3, _ := stateRepo3.Load()

		stateRepo4 := env.ProcessMap["4"].Services["StateRepository"].(*mem.StateRepository)
		state4, _ := stateRepo4.Load()



		stateApi1.StartConsensus(pbft.ProposedBlock{Seal: []byte{'s', 'd', 'f'}, Body: []byte{'2', '3', '3'}})

		time.Sleep(5 * time.Second)

		iLogger.Infof(nil,"SEAL", state1.Block.Seal)
		assert.Equal(t,state1.Block.Seal,state2.Block.Seal)
		assert.Equal(t,state2.Block.Seal,state3.Block.Seal)
		assert.Equal(t,state3.Block.Seal,state4.Block.Seal)

	}
}

func TestConsensusApi_HandleProposeMsg(t *testing.T) {

	var validLeaderProposeMsg = pbft.ProposeMsg{
		StateID: pbft.StateID{
			ID: "state1",
		},
		SenderID:       "user0",
		Representative: nil,
		ProposedBlock: pbft.ProposedBlock{
			Seal: make([]byte, 0),
			Body: make([]byte, 0),
		},
	}
	var invalidLeaderProposeMsg = pbft.ProposeMsg{
		StateID: pbft.StateID{
			ID: "state1",
		},
		SenderID:       "",
		Representative: nil,
		ProposedBlock:  pbft.ProposedBlock{},
	}
	tests := map[string]struct {
		input struct {
			proposeMsg pbft.ProposeMsg
			peerNum    int
			isRepoFull bool
		}
		err error
	}{
		"Case 1 PrePrepareMsg의 Sender id와 Request된 Leader id가 일치하며, repo가 차있는 경우 (Normal Case)": {
			input: struct {
				proposeMsg pbft.ProposeMsg
				peerNum    int
				isRepoFull bool
			}{validLeaderProposeMsg, 5, false},
			err: nil,
		},
		"Case 2 PrePrepareMsg의 Sender id와 Request된 Leader id가 일치하며, repo가 차있는 경우": {
			input: struct {
				proposeMsg pbft.ProposeMsg
				peerNum    int
				isRepoFull bool
			}{validLeaderProposeMsg, 5, true},
			err: pbft.ErrInvalidSave,
		},
		"Case 3 PrePrepareMsg의 Sender id와 Request된 Leader id가 일치하지 않을 경우": {
			input: struct {
				proposeMsg pbft.ProposeMsg
				peerNum    int
				isRepoFull bool
			}{invalidLeaderProposeMsg, 5, false},
			err: pbft.InvalidLeaderIdError,
		},
	}

	for testName, test := range tests {
		t.Logf("running test case %s ", testName)
		cApi := setUpApiCondition(test.input.peerNum, test.input.isRepoFull, true)
		assert.EqualValues(t, test.err, cApi.HandleProposeMsg(test.input.proposeMsg))

	}
}

func TestConsensusApi_HandlePrevoteMsg(t *testing.T) {

	var validPrevoteMsg = pbft.PrevoteMsg{
		StateID:   pbft.StateID{"state"},
		SenderID:  "user1",
		BlockHash: []byte{1, 2, 3, 5},
	}
	var invalidPrevoteMsg = pbft.PrevoteMsg{
		StateID:   pbft.StateID{"invalidState"},
		SenderID:  "user1",
		BlockHash: []byte{1, 2, 3, 5},
	}

	tests := map[string]struct {
		input struct {
			prevoteMsg pbft.PrevoteMsg
			peerNum    int
			isRepoFull bool
		}
		err error
	}{
		"Case 1 PrepareMsg의 Cid와 repo의 Cid가 같고, repo에 consensus가 저장된경우 (Normal Case)": {
			input: struct {
				prevoteMsg pbft.PrevoteMsg
				peerNum    int
				isRepoFull bool
			}{validPrevoteMsg, 5, true},
			err: nil,
		},
		"Case 2 PrepareMsg의 Cid와 repo에 저장된 Cid가 다를 경우": {
			input: struct {
				prevoteMsg pbft.PrevoteMsg
				peerNum    int
				isRepoFull bool
			}{invalidPrevoteMsg, 5, true},
			err: pbft.ErrStateIdNotSame,
		},
		"Case 3 Repo의 state가 empty state일때": {
			input: struct {
				prevoteMsg pbft.PrevoteMsg
				peerNum    int
				isRepoFull bool
			}{invalidPrevoteMsg, 5, false},
			err: pbft.ErrEmptyRepo,
		},
	}

	for testName, test := range tests {
		t.Logf("running test case %s ", testName)
		cApi := setUpApiCondition(test.input.peerNum, test.input.isRepoFull, true)
		assert.EqualValues(t, test.err, cApi.HandlePrevoteMsg(test.input.prevoteMsg))
	}

}

func TestConsensusApi_HandlePreCommitMsg(t *testing.T) {

	var validCommitMsg = pbft.PreCommitMsg{
		StateID:  pbft.StateID{"state"},
		SenderID: "user1",
	}
	var invalidCommitMsg = pbft.PreCommitMsg{
		StateID:  pbft.StateID{"invalidState"},
		SenderID: "user2",
	}

	tests := map[string]struct {
		input struct {
			commitMsg     pbft.PreCommitMsg
			peerNum       int
			isRepoFull    bool
			isNormalBlock bool
		}
		err error
	}{
		"Case 1 repo에 consensus가 있고 repo의 cid와 commitMsg의 cid가 일치하는 경우(Normal Case)": {
			input: struct {
				commitMsg     pbft.PreCommitMsg
				peerNum       int
				isRepoFull    bool
				isNormalBlock bool
			}{validCommitMsg, 5, true, true},
			err: nil,
		},
		"Case 2 repo에 저장된 state의 cid와 commitMsg의 cid가 일치하지 않은 경우": {
			input: struct {
				commitMsg     pbft.PreCommitMsg
				peerNum       int
				isRepoFull    bool
				isNormalBlock bool
			}{invalidCommitMsg, 5, true, true},
			err: pbft.ErrStateIdNotSame,
		},
		"Case 3 repo에 저장된 state가 empty state일때": {
			input: struct {
				commitMsg     pbft.PreCommitMsg
				peerNum       int
				isRepoFull    bool
				isNormalBlock bool
			}{validCommitMsg, 5, false, false},
			err: pbft.ErrEmptyRepo,
		},
	}

	for testName, test := range tests {
		t.Logf("running test case %s ", testName)
		cApi := setUpApiCondition(test.input.peerNum, test.input.isRepoFull, test.input.isNormalBlock)
		assert.EqualValues(t, test.err, cApi.HandlePreCommitMsg(test.input.commitMsg))
	}
}

func setUpApiCondition(peerNum int, isRepoFull bool, isNormalBlock bool) *api.StateApiImpl {

	reps := make([]pbft.Representative, 0)
	for i := 0; i < 6; i++ {
		reps = append(reps, pbft.Representative{
			ID: "user",
		})
	}

	commitMsgPool := pbft.NewPreCommitMsgPool()
	for i := 0; i < 5; i++ {
		senderStr := "sender"
		senderStr += string(i)
		commitMsgPool.Save(&pbft.PreCommitMsg{
			StateID:  pbft.StateID{"state"},
			SenderID: senderStr,
		})
	}

	mockEventService := mock.EventService{}
	mockEventService.PublishFunc = func(topic string, event interface{}) error {
		return nil
	}

	propagateService := pbft.NewPropagateService(mockEventService)
	parliamentRepository := mem.NewParliamentRepository()
	parliament := pbft.NewParliament()
	for i := 0; i < peerNum; i++ {
		userStr := "user"
		userStr += strconv.Itoa(i)
		parliament.AddRepresentative(pbft.NewRepresentative(userStr))
	}

	parliament.SetLeader("user0")
	parliamentRepository.Save(parliament)

	eventService := mock.EventService{}
	eventService.PublishFunc = func(topic string, event interface{}) error {
		return nil
	}
	eventService.ConfirmBlockFunc = func(block pbft.ProposedBlock) error {
		return nil
	}

	repo := mem.NewStateRepository()
	if isRepoFull && isNormalBlock {

		savedConsensus := pbft.State{
			StateID:          pbft.StateID{"state"},
			Representatives:  reps,
			Block:            normalBlock,
			CurrentStage:     pbft.IDLE_STAGE,
			PrevoteMsgPool:   pbft.PrevoteMsgPool{},
			PreCommitMsgPool: pbft.PreCommitMsgPool{},
		}
		repo.Save(savedConsensus)

	} else if isRepoFull && !isNormalBlock {
		savedConsensus := pbft.State{
			StateID:          pbft.StateID{"state"},
			Representatives:  reps,
			Block:            errorBlock,
			CurrentStage:     pbft.IDLE_STAGE,
			PrevoteMsgPool:   pbft.PrevoteMsgPool{},
			PreCommitMsgPool: pbft.PreCommitMsgPool{},
		}
		repo.Save(savedConsensus)
	}

	cApi := api.NewStateApi("my", propagateService, eventService, parliamentRepository, repo)
	return cApi
}
