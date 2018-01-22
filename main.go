package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"

	"github.com/byuoitav/configuration-database-microservice/structs"
	"github.com/byuoitav/room-duplication/db"
	"github.com/fatih/color"
)

func main() {

	var newRoom = flag.String("newroom", "", "New room name i.e. ITB-1101")
	var oldRoom = flag.String("oldroom", "", "Room name to be copied i.e. ITB-1102")
	var useDNS = flag.Bool("usedns", true, "Use DNS to resolve the FQDN (address) of the devices")
	var duplicateUI = flag.Bool("dupui", true, "Duplicate the ui configuration")

	flag.Parse()

	if len(*newRoom) <= 0 || len(*oldRoom) <= 0 {
		log.Printf("Cannot continue without old room and new room information...")
		return
	}

	log.Printf("Calling")
	db.Start()
	duplicateRoom(*newRoom, *oldRoom, *duplicateUI, *useDNS)
}

func duplicateRoom(NR, OR string, dUI, uDNS bool) {
	log.Printf(color.HiGreenString("Starting duplication of room %v to %v", OR, NR))

	newSplit := strings.Split(NR, "-")
	oldSplit := strings.Split(OR, "-")

	//get the information from the old room
	building, err := db.AG.GetBuildingByShortname(newSplit[0])
	if err != nil {
		log.Printf("Error: %v", err.Error())
		return
	}
	if len(building.Shortname) <= 0 {
		log.Printf("No Building by that name: %v", newSplit[0])
		return
	}

	//get the old room
	room, err := db.AG.GetRoomByBuildingAndName(oldSplit[0], oldSplit[1])
	if err != nil {
		log.Printf("Error: %v", err.Error())
		return
	}
	if len(room.Name) <= 0 {
		log.Printf("No Room by that name: %v", oldSplit[1])
		return
	}

	room.Name = newSplit[1]

	room, err = db.AG.AddRoom(newSplit[0], room)
	if err != nil {
		log.Printf("Error: %v", err.Error())
		return
	}
	log.Printf("newRoomID: %v", room.ID)

	tempRoom := room
	tempRoom.Devices = []structs.Device{}

	//build our mapping now so that we can go back and do our port stuff later
	//old device -> new device
	deviceMapping := make(map[int]int)

	//now we have the new roomID, we can go through the devices for that room, and replicate them
	for _, dev := range room.Devices {
		log.Printf("Working on device %v", dev.GetFullName())

		oldID := dev.ID
		dev.Room = tempRoom
		dev.Address = "0.0.0.0"

		if uDNS {
			dev.Address = lookupAddress(dev.GetFullName())
		}
		dev, err = db.AG.AddDevice(dev)
		if err != nil {
			log.Printf("Error: %v", err.Error())
		}
		log.Printf(color.HiGreenString("new ID: %v", dev.ID))
		newID := dev.ID
		deviceMapping[oldID] = newID
	}

	//now that we've added all the devices (With roles and power states) we just need to figure out the ports

	copyPortMappings(deviceMapping)
	log.Printf(color.HiGreenString("Done copying room."))

	if !dUI {
		return
	}
	log.Printf(color.HiGreenString("Copying the ui configuration"))

	//now we just need to copy the ui-configuration
	//go to the directory
	uidir := os.Getenv("UI_CONFIGURATION_DIRECTORY")

	//copy the old ui
	b, err := ioutil.ReadFile(fmt.Sprintf("%s/%s/config.json", uidir, OR))
	if err != nil {
		log.Printf(color.HiRedString("Problem reading the config ui to copy: %v", err.Error()))
		return
	}

	b = bytes.Replace(b, []byte(OR), []byte(NR), -1)

	os.MkdirAll(fmt.Sprintf("%s/%s", uidir, NR), 0755)
	//write out the new one
	err = ioutil.WriteFile(fmt.Sprintf("%s/%s/config.json", uidir, NR), b, 0755)
	if err != nil {
		log.Printf(color.HiRedString("Problem writing the copied config ui: %v", err.Error()))
	}

	//done
	log.Printf(color.HiGreenString("Done copying UI configuration."))
	log.Printf(color.HiGreenString("Success!"))
}

func lookupAddress(name string) string {
	log.Printf(color.HiMagentaString("Looking up address for: %v", name))

	addr, err := net.LookupIP(name)
	if err != nil {
		//device does not exist in qip
		log.Printf(color.HiRedString("Addr not in DNS resolvers for %v", name))
		return "0.0.0.0"
	}

	//get the dns name now

	host, err := net.LookupAddr(addr[0].String())
	if err != nil {
		log.Printf(color.HiRedString("Issue finding DNS name", name))
		return addr[0].String()
	}

	//remove the last .
	return host[0][:len(host)-1]
}

func copyPortMappings(mappings map[int]int) error {

	log.Printf(color.HiGreenString("Starting port mapping"))
	log.Printf(color.CyanString("Mappings:"))

	for k, v := range mappings {
		log.Printf(color.CyanString("%v\t->\t%v", k, v))
	}

	for k, v := range mappings {
		ports, err := db.AG.GetPortsByHostID(k)
		if err != nil {
			log.Printf(color.HiRedString("Could not get ports for deviceID: %v %v", k, err.Error()))
			continue
		}
		if len(ports) <= 0 {
			log.Printf(color.HiBlueString("No ports for device %v", k))
			continue
		}

		//now we go through and do our mappings
		for p := range ports {
			ports[p].HostDeviceID = v
			ports[p].SourceDeviceID = mappings[ports[p].SourceDeviceID]
			ports[p].DestinationDeviceID = mappings[ports[p].DestinationDeviceID]

			log.Printf(color.BlueString("adding port %+v", ports[p]))

			_, err = db.AG.AddPortConfiguration(ports[p])
			if err != nil {
				log.Printf(color.HiRedString("Error adding ports: %v", err.Error()))
			}
		}

	}
	log.Printf(color.HiGreenString("Done doing port mapping"))
	return nil
}
