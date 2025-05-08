package rior

type Rior interface {
	Get(name string) string
	GetDS(name string) Rior
}
