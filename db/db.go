package db

import (
	"github.com/byuoitav/configuration-database-microservice/accessors"

	"os"
)

var database = ""
var AG *accessors.AccessorGroup

func Start() {
	database = os.Getenv("CONFIGURATION_DATABASE_USERNAME") + ":" + os.Getenv("CONFIGURATION_DATABASE_PASSWORD") + "@tcp(" + os.Getenv("CONFIGURATION_DATABASE_HOST") + ":" + os.Getenv("CONFIGURATION_DATABASE_PORT") + ")/" + os.Getenv("CONFIGURATION_DATABASE_NAME")

	AG = new(accessors.AccessorGroup)
	AG.Open(database)
}
