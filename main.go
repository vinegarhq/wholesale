package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/apprehensions/rbxbin"
	cs "github.com/apprehensions/rbxweb/clientsettings"
	"github.com/vinegarhq/wholesale/internal/factory"
)

func main() {
	guid := flag.String("guid", "", "Roblox deployment GUID to retrieve")
	channel := flag.String("channel", "LIVE", "Roblox deployment channel for the GUID")
	bin := flag.String("type", "WindowsPlayer", "Roblox BinaryType for the GUID")
	flag.Parse()

	if len(os.Args) < 2 {
		usage()
	}

	var t cs.BinaryType
	switch *bin {
	case "WindowsPlayer":
		t = cs.WindowsPlayer
	case "WindowsStudio64":
		t = cs.WindowsStudio64
	default:
		log.Fatal("Unsupported binary type", *bin,
			"must be either WindowsPlayer or WindowsStudio64")
	}

	d := rbxbin.Deployment{
		Type:    t,
		Channel: *channel,
		GUID:    *guid,
	}

	ba := factory.NewBinaryAssembler(d, &humanResources{})
	buf := ba.Run()

	link(buf, fmt.Sprintf(
		"%s-%s-%s.zip", d.Type, d.Channel, d.GUID))
}
