package magellan

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

type UpdateParams struct {
	CollectParams
	FirmwarePath     string
	FirmwareVersion  string
	Component        string
	TransferProtocol string
	Insecure         bool
}

// FileCopy copies a single file from src to dst
func FileCopy(src string, dst string) error {

	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	foo, err := io.Copy(destination, source)
	fmt.Println(foo)
	return err
}

// UpdateFirmwareRemote() uses 'gofish' to update the firmware of a BMC node.
// The function expects the firmware URL, firmware version, and component flags to be
// set from the CLI to perform a firmware update.
// Example:
// ./magellan update https://192.168.23.40 --username root --password 0penBmc
// --firmware-url http://192.168.23.19:1337/obmc-phosphor-image.static.mtd.tar
// --scheme TFTP
//
// being:
// q.URI https://192.168.23.40
// q.TransferProtocol TFTP
// q.FirmwarePath http://192.168.23.19:1337/obmc-phosphor-image.static.mtd.tar
func UpdateFirmwareRemote(q *UpdateParams) error {
	// parse URI to set up full address
	uri, err := url.ParseRequestURI(q.URI)
	if err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}

	if !(q.Insecure) {
		// SECURE MODE
		fmt.Println("SECURE MODE (OpenBMC mode https://github.com/openbmc)")
		// first to copy in local the file that we want to uplaod to the BMC
		currentDir, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
		}
		destPath := filepath.Join(currentDir, "newfw.mtd.tar")
		FileCopy(q.FirmwarePath, destPath)

		// On OpenBMC we need to execute this kind of command
		// 	curl  -k -H "X-Auth-Token: $token"  -H "Content-Type:multipart/form-data" -X POST \
		//  -F UpdateParameters='{"Targets":["/redfish/v1/Managers/bmc"],"@Redfish.OperationApplyTime":"Immediate"};type=application/json' \
		//  -F 'UpdateFile=@obmc-phosphor-image-sipearl-evb0-20250304115722.static.mtd.tar;type=application/octet-stream' \
		// https://root:0penBmc@192.168.75.39/redfish/v1/UpdateService/update

		curlcmd := "curl"

		prefix := "//" + q.Username + ":" + q.Password + "@"
		theurl := strings.Replace(uri.String(), "//", prefix, 1)
		theurl += "/redfish/v1/UpdateService/update"
		cmd := exec.Command(curlcmd, "-k", "-H \"X-Auth-Token: $token\"",
			"-H \"Content-Type:multipart/form-data\"", "-X", "POST",
			"-F", "UpdateFile=@newfw.mtd.tar;type=application/octet-stream",
			"-F", "UpdateParameters={\"Targets\":[\"/redfish/v1/Managers/bmc\"],\"@Redfish.OperationApplyTime\":\"Immediate\"}",
			theurl)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			fmt.Printf("cmd.Run() failed with %s\n", err)
		} else {
			fmt.Println("Firmware update initiated successfully.")
		}
		// remove the copy of the BMC image
		err = os.Remove("newfw.mtd.tar")
		if err != nil {
			fmt.Printf("os.Remove failed with %s\n", err)
		}

		return nil
	}

	// INSECURE MODE
	// Connect to the Redfish service using gofish
	client, err := gofish.Connect(gofish.ClientConfig{Endpoint: uri.String(), Username: q.Username, Password: q.Password, Insecure: q.Insecure})
	if err != nil {
		return fmt.Errorf("failed to connect to Redfish service: %w", err)
	}
	defer client.Logout()

	// Retrieve the UpdateService from the Redfish client
	updateService, err := client.Service.UpdateService()
	if err != nil {
		return fmt.Errorf("failed to get update service: %w", err)
	}

	// Build the update request payload
	req := redfish.SimpleUpdateParameters{
		ImageURI:         q.FirmwarePath,
		TransferProtocol: redfish.TransferProtocolType(q.TransferProtocol),
	}

	// Execute the SimpleUpdate action
	err = updateService.SimpleUpdate(&req)
	if err != nil {
		return fmt.Errorf("firmware update failed: %w", err)
	}
	fmt.Println("Firmware update initiated successfully.")
	return nil
}

func GetUpdateStatus(q *UpdateParams) error {
	// parse URI to set up full address
	uri, err := url.ParseRequestURI(q.URI)
	if err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}

	// Connect to the Redfish service using gofish
	client, err := gofish.Connect(gofish.ClientConfig{Endpoint: uri.String(), Username: q.Username, Password: q.Password, Insecure: q.Insecure})
	if err != nil {
		return fmt.Errorf("failed to connect to Redfish service: %w", err)
	}
	defer client.Logout()

	// Retrieve the UpdateService from the Redfish client
	updateService, err := client.Service.UpdateService()
	if err != nil {
		return fmt.Errorf("failed to get update service: %w", err)
	}

	// Get the update status
	status := updateService.Status
	fmt.Printf("Update Status: %v\n", status)

	return nil
}
