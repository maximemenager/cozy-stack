// go run fsck.go
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	dt := time.Now()
	inFile := "instances.txt"
	outFileData := fmt.Sprintf("/tmp/out.%s.txt", dt.Format("01-02-2006"))
	outFileInstancesClean := fmt.Sprintf("/tmp/instancesClean.%s.txt", dt.Format("01-02-2006"))
	outFileInstancesCorrupted := fmt.Sprintf("/tmp/instancesCorrupted.%s.txt", dt.Format("01-02-2006"))

	f, _ := os.Open(inFile)
	outFile, _ := os.OpenFile(outFileData, os.O_CREATE|os.O_WRONLY, 0644)
	outFileI, _ := os.OpenFile(outFileInstancesClean, os.O_CREATE|os.O_WRONLY, 0644)
	outFileC, _ := os.OpenFile(outFileInstancesCorrupted, os.O_CREATE|os.O_WRONLY, 0644)

	defer f.Close()
	defer outFile.Close()
	defer outFileI.Close()
	defer outFileC.Close()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	var instances []string
	for scanner.Scan() { // Instance
		var t []string
		instanceName := scanner.Text()
		fmt.Printf("Working on %s\n", instanceName)

		// Executing the command
		output, err := exec.Command("cozy-stack", "instances", "fsck", instanceName, "--json").Output()

		if len(output) == 0 {
			s := fmt.Sprintf("%s\n", instanceName)
			outFileI.WriteString(s)
		} else {
			outFileC.WriteString(fmt.Sprintf("%s\n", instanceName))
			// Reading the command return output
			scan := bufio.NewScanner(bytes.NewReader(output))
			scan.Split(bufio.ScanLines)

			for scan.Scan() { // Issue line
				if err != nil {
					os.Stderr.WriteString(err.Error())
				} else {
					t = append(t, scan.Text())
				}
			}

			// Append the instance
			joinedLines := strings.Join(t, ",")
			instance := fmt.Sprintf("\"%s\":[%s]", instanceName, joinedLines)
			instances = append(instances, instance)
		}
	}

	joinedInstances := strings.Join(instances, ",")
	jsonInstances := fmt.Sprintf("{%s}", joinedInstances)
	outFile.WriteString(jsonInstances)

	return
}
