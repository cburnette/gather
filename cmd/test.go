package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

type output struct {
	deviceID int
	device   string
	command  string
	output   string
}

var mutex = &sync.Mutex{}
var results []output

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func doTest(cmd *cobra.Command, args []string) {
	outputFile, err := rootCmd.PersistentFlags().GetString("output")
	if err != nil {
		panic(err)
	}

	if outputFile == defaultOutputFile {
		outputFile = fmt.Sprintf("gather-%s.txt", time.Now().UTC().Format(time.RFC3339))
	}
	fmt.Printf("Output File:\n")
	fmt.Printf("%s\n\n", outputFile)

	devices := getDevices()
	fmt.Printf("Devices:\n")
	for _, device := range devices {
		fmt.Println(device)
	}

	commands := getCommands()
	fmt.Printf("\nCommands:\n")
	for _, command := range commands {
		fmt.Println(command)
	}

	var user string
	fmt.Print("\nuser: ")
	fmt.Scanf("%s", &user)

	fmt.Print("password: ")
	password, err := terminal.ReadPassword(0)
	if err != nil {
		panic(err)
	}
	fmt.Println()

	//resultsChannel := make(chan output)
	var wg sync.WaitGroup
	wg.Add(len(devices))

	debug, err := rootCmd.PersistentFlags().GetBool("debug")
	if err != nil {
		panic(err)
	}

	for t := 0; t < len(devices); t++ {
		if !debug {
			go execCommands(t, user, string(password), devices[t], commands, &wg)
		} else {
			execCommands(t, user, string(password), devices[t], commands, &wg)
		}
	}

	// go func() {
	// 	for v := range resultsChannel {
	// 		results = append(results, v)
	// 	}
	// }()

	for len(results) < len(devices)*len(commands) {
		wg.Wait()
	}

	//close(resultsChannel)

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].deviceID < results[j].deviceID
	})

	// fmt.Printf("\n\nOutput\n")
	// for r := range results {
	// 	scanner := bufio.NewScanner(strings.NewReader(results[r].output))
	// 	for scanner.Scan() {
	// 		fmt.Printf("%s %s %s %s %s\n", results[r].device, separator, results[r].command, separator, scanner.Text())
	// 	}
	// }

	writeOutputFile(results, outputFile)
}

func addResult(o output) {
	mutex.Lock()
	results = append(results, o)
	mutex.Unlock()
}

func writeOutputFile(results []output, outputFile string) {
	f, err := os.Create(outputFile)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	for r := range results {
		scanner := bufio.NewScanner(strings.NewReader(results[r].output))
		for scanner.Scan() {
			line := fmt.Sprintf("%s %s %s %s %s\n", results[r].device, separator, results[r].command, separator, scanner.Text())
			_, err := f.WriteString(line)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func execCommands(deviceID int, user string, password string, device string, commands []string, wg *sync.WaitGroup) {
	defer wg.Done()

	client, err := connectToDevice(user, password, device)
	if err != nil {
		for c := 0; c < len(commands); c++ {
			output := output{
				deviceID: deviceID,
				device:   device,
				command:  commands[c],
				output:   err.Error(),
			}
			//resultsChannel <- output
			addResult(output)
		}
		log.Printf(err.Error())
		return
	}

	defer client.Close()

	for c := 0; c < len(commands); c++ {
		session, err := client.NewSession()

		if err != nil {
			output := output{
				deviceID: deviceID,
				device:   device,
				command:  commands[c],
				output:   err.Error(),
			}
			//resultsChannel <- output
			addResult(output)
			log.Printf("%s %s %s %s %s\n", device, separator, commands[c], separator, err.Error())
			//log.Printf("error creating session for command %s on device %s", commands[c], device)
			session.Close()
			continue
		}

		var b bytes.Buffer
		session.Stdout = &b
		if err := session.Run(commands[c]); err != nil {
			//log.Fatal("Failed to run: " + err.Error())
			output := output{
				deviceID: deviceID,
				device:   device,
				command:  commands[c],
				output:   err.Error(),
			}
			//resultsChannel <- output
			addResult(output)
			log.Printf("%s %s %s %s %s\n", device, separator, commands[c], separator, err.Error())
			//log.Printf("error executing command %s on device %s", commands[c], device)
			session.Close()
			continue
		}
		//fmt.Println(b.String())

		// out, err := session.Output(commands[c])
		// if err != nil {
		// 	output := output{
		// 		deviceID: deviceID,
		// 		device:   device,
		// 		command:  commands[c],
		// 		output:   err.Error(),
		// 	}
		// 	//resultsChannel <- output
		// 	addResult(output)
		// 	log.Printf("%s %s %s %s %s\n", device, separator, commands[c], separator, err.Error())
		// 	//log.Printf("error executing command %s on device %s", commands[c], device)
		// 	session.Close()
		// 	continue
		// }

		output := output{
			deviceID: deviceID,
			device:   device,
			command:  commands[c],
			output:   string(b.String()),
		}

		//resultsChannel <- output
		addResult(output)
		session.Close()
	}
}

func connectToDevice(user, password, device string) (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", device, sshConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func getDevices() []string {
	deviceFile, err := rootCmd.PersistentFlags().GetString("devices")
	if err != nil {
		panic(err)
	}

	file, err := os.Open(deviceFile)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var devices []string

	for scanner.Scan() {
		device := scanner.Text()
		if !strings.HasPrefix(device, "#") {
			parts := strings.Split(device, ":")
			if len(parts) == 1 {
				device = device + ":22"
			}
			devices = append(devices, device)
		}
	}

	return devices
}

func getCommands() []string {
	commandFile, err := rootCmd.PersistentFlags().GetString("commands")
	if err != nil {
		panic(err)
	}

	file, err := os.Open(commandFile)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var commands []string

	for scanner.Scan() {
		command := scanner.Text()
		if !strings.HasPrefix(command, "#") {
			commands = append(commands, command)
		}
	}

	return commands
}
