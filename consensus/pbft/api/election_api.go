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

package api

import (
	"strings"
	"time"

	"github.com/it-chain/engine/common"
	"github.com/it-chain/engine/common/command"
	"github.com/it-chain/engine/common/logger"
	"github.com/it-chain/engine/consensus/pbft"
	"github.com/it-chain/iLogger"
	"github.com/rs/xid"
)

type ElectionApi struct {
	ElectionService      *pbft.ElectionService
	parliamentRepository pbft.ParliamentRepository
	eventService         common.EventService
	quit                 chan struct{}
}

func NewElectionApi(electionService *pbft.ElectionService, parliamentRepository pbft.ParliamentRepository, eventService common.EventService) *ElectionApi {

	return &ElectionApi{
		ElectionService:      electionService,
		parliamentRepository: parliamentRepository,
		eventService:         eventService,
		quit:                 make(chan struct{}, 1),
	}
}

func (e *ElectionApi) Vote(connectionId string) error {

	parliament := e.parliamentRepository.Load()

	representative, err := parliament.FindRepresentativeByID(connectionId)
	if err != nil {
		iLogger.Infof(nil, "[PBFT] Representative who has Id: %s is not found", connectionId)
		return err
	}

	e.ElectionService.SetCandidate(representative)
	e.ElectionService.ResetLeftTime()

	voteLeaderMessage := pbft.VoteMessage{}
	grpcDeliverCommand, _ := CreateGrpcDeliverCommand("VoteLeaderProtocol", voteLeaderMessage)
	grpcDeliverCommand.RecipientList = append(grpcDeliverCommand.RecipientList, connectionId)

	iLogger.Infof(nil, "[PBFT] Vote to %s", connectionId)
	e.ElectionService.SetVoted(true)

	return e.eventService.Publish("message.deliver", grpcDeliverCommand)
}

// broadcast leader to other peers
func (e *ElectionApi) broadcastLeader(rep pbft.Representative) error {
	iLogger.Infof(nil, "[PBFT] Broadcast leader id: %s", rep.ID)

	updateLeaderMessage := pbft.UpdateLeaderMessage{
		Representative: rep,
	}
	grpcDeliverCommand, err := CreateGrpcDeliverCommand("UpdateLeaderProtocol", updateLeaderMessage)
	if err != nil {
		iLogger.Infof(nil, "[Consensus] Err %s", err.Error())
		return err
	}

	parliament := e.parliamentRepository.Load()
	for _, r := range parliament.GetRepresentatives() {
		grpcDeliverCommand.RecipientList = append(grpcDeliverCommand.RecipientList, r.ID)
	}

	return e.eventService.Publish("message.deliver", grpcDeliverCommand)
}

//broadcast leader when voted fully
func (e *ElectionApi) DecideToBeLeader() error {
	iLogger.Infof(nil, "[PBFT] Receive vote")
	if e.ElectionService.GetState() != pbft.CANDIDATE {
		return nil
	}

	e.ElectionService.CountUpVoteCount()

	if e.isFullyVoted() {
		iLogger.Infof(nil, "[PBFT] Leader has fully voted")

		e.EndRaft()
		representative := pbft.Representative{
			ID: e.ElectionService.NodeId,
		}

		e.eventService.Publish("leader.updated", e.ElectionService.NodeId)
		if err := e.broadcastLeader(representative); err != nil {
			return err
		}
	}

	return nil
}

func (e *ElectionApi) isFullyVoted() bool {
	parliament := e.parliamentRepository.Load()
	numOfPeers := len(parliament.Representatives)
	if e.ElectionService.GetVoteCount() == numOfPeers-1 {
		return true
	}

	return false
}

//1. Start random timeout
//2. timed out! alter state to 'candidate'
//3. while ticking, count down leader repo left time
//4. Send message having 'RequestVoteProtocol' to other node
func (e *ElectionApi) ElectLeaderWithRaft() {

	e.ElectionService.SetState(pbft.TICKING)
	e.ElectionService.InitLeftTime()
	tick := time.Tick(1 * time.Millisecond)
	timeout := time.After(time.Second * 10)

	for {
		select {
		case <-tick:
			e.ElectionService.CountDownLeftTimeBy(1)
			if e.ElectionService.GetLeftTime() == 0 {
				e.HandleRaftTimeout()
			}
		case <-e.quit:
			logger.Infof(nil, "[PBFT] Raft has end")
			return
		case <-timeout:
			logger.Errorf(nil, "[PBFT] Raft Time out")
			return
		}
	}
}

func (e *ElectionApi) EndRaft() {
	e.quit <- struct{}{}
}

func (e *ElectionApi) HandleRaftTimeout() error {
	if e.ElectionService.GetState() == pbft.TICKING {
		e.ElectionService.SetState(pbft.CANDIDATE)
		connectionIds := make([]string, 0)
		parliament := e.parliamentRepository.Load()
		for _, r := range parliament.GetRepresentatives() {
			connectionIds = append(connectionIds, r.ID)
		}
		e.RequestVote(connectionIds)

	} else if e.ElectionService.GetState() == pbft.CANDIDATE {
		//reset time and state chane candidate -> ticking when timed in candidate state
		e.ElectionService.ResetLeftTime()
		e.ElectionService.SetState(pbft.TICKING)
	}

	return nil
}
func (e *ElectionApi) RequestVote(peerIds []string) error {

	iLogger.Infof(nil, "[PBFT] Request Vote - Peers:[%s]", strings.Join(peerIds, ", "))
	// 1. create request vote message
	// 2. send message
	requestVoteMessage := pbft.RequestVoteMessage{
		Term: e.ElectionService.GetTerm(),
	}
	grpcDeliverCommand, _ := CreateGrpcDeliverCommand("RequestVoteProtocol", requestVoteMessage)

	for _, connectionId := range peerIds {
		grpcDeliverCommand.RecipientList = append(grpcDeliverCommand.RecipientList, connectionId)
	}
	return e.eventService.Publish("message.deliver", grpcDeliverCommand)
}

func (e *ElectionApi) GetCandidate() pbft.Representative {
	return e.ElectionService.GetCandidate()
}

func (e *ElectionApi) GetState() pbft.ElectionState {
	return e.ElectionService.GetState()
}

func (e *ElectionApi) SetState(state pbft.ElectionState) {
	e.ElectionService.SetState(state)
}

func (e *ElectionApi) GetVoteCount() int {
	return e.ElectionService.GetVoteCount()
}

func CreateGrpcDeliverCommand(protocol string, body interface{}) (command.DeliverGrpc, error) {

	data, err := common.Serialize(body)

	if err != nil {
		return command.DeliverGrpc{}, err
	}

	return command.DeliverGrpc{
		MessageId:     xid.New().String(),
		RecipientList: make([]string, 0),
		Body:          data,
		Protocol:      protocol,
	}, err
}
