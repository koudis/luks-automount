package worker

const (
	ExitOK       = 0
	ExitOpError  = 1
	ExitProtocol = 2
)

type Response struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}
