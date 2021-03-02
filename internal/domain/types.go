package domain

type Identity string

type ClientID string

func (cid ClientID) String() string {
	return string(cid)
}

type AccountID string

func (aid AccountID) String() string {
	return string(aid)
}
