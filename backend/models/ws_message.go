package models

type WSMessageType string

const (
	WSMsgInitState    WSMessageType = "INIT_STATE"
	WSMsgVoteUpdate   WSMessageType = "VOTE_UPDATE"
	WSMsgViewerUpdate WSMessageType = "VIEWER_UPDATE"
	WSMsgPollClosed   WSMessageType = "POLL_CLOSED"
	WSMsgPollDeleted  WSMessageType = "POLL_DELETED"
	WSMsgError        WSMessageType = "ERROR"
)

type WSMessage struct {
	Type        WSMessageType    `json:"type"`
	PollID      string           `json:"pollId,omitempty"`
	Votes       map[string]int64 `json:"votes,omitempty"`
	TotalVotes  int64            `json:"totalVotes,omitempty"`
	ViewerCount int              `json:"viewerCount,omitempty"`
	IsClosed    bool             `json:"isClosed,omitempty"`
	IsDeleted   bool             `json:"isDeleted,omitempty"`
	OptionID    string           `json:"optionId,omitempty"`
	Error       string           `json:"error,omitempty"`
}
