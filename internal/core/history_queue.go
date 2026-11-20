package core

type HistoryPageRequest struct {
	MixedOnly      bool `json:"mixedOnly,omitempty"`
	QueueID        int  `json:"queueId"`
	Offset         int  `json:"offset"`
	MixedOffset    int  `json:"mixedOffset"`
	RemainingPages int  `json:"remainingPages"`
}

type HistoryQueueVerification struct {
	QueueID  int    `json:"queueId"`
	State    string `json:"state"`
	Mixed    int    `json:"mixed"`
	Filtered int    `json:"filtered"`
	Compared int    `json:"compared"`
}
