package monitor

type Action int

const (
	ActionAdd Action = iota
	ActionRemove
)

func (a Action) String() string {
	switch a {
	case ActionAdd:
		return "add"
	case ActionRemove:
		return "remove"
	}
	return "unknown"
}

type Event struct {
	Action  Action
	DevPath string
	DevName string
}
