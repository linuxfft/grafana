package sqlstore

import (
	"github.com/masami10/grafana/pkg/bus"
	m "github.com/masami10/grafana/pkg/models"
)

func init() {
	bus.AddHandler("sql", GetDBHealthQuery)
}

func GetDBHealthQuery(query *m.GetDBHealthQuery) error {
	return x.Ping()
}
