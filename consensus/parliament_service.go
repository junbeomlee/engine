package consensus

import (
	"errors"
)

type ParliamentServiceImpl struct {

}

func NewParliamentSerivce() ParliamentService {
	return &ParliamentServiceImpl{}
}

func (ps ParliamentServiceImpl) Elect(parliament Parliament) ([]*Representative, error) {
	representatives := make([]*Representative, 0)

	if !parliament.HasLeader() {
		return nil, errors.New("No Leader")
	}

	representatives = append(representatives, NewRepresentative(parliament.Leader.GetID()))

	for _, member := range parliament.Members {
		representatives = append(representatives, NewRepresentative(member.GetID()))
	}

	return representatives, nil
}