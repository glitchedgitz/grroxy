package types


type BaseCollection struct {
	Id         string
	Name       string
	Type       string
	Schema     any
	System     bool
	ListRule   string
	ViewRule   string
	CreateRule string
	UpdateRule string
	DeleteRule string
}

type ParamsList struct {
	Page    int
	Size    int
	Filters string
	Sort    string

	HackResponseRef any //hack for collection list
}
