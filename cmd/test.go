package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"

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

	var results []output
	resultsChannel := make(chan output)
	var wg sync.WaitGroup

	for t := 0; t < len(devices); t++ {
		wg.Add(1)
		go execCommands(t, user, string(password), devices[t], commands, &wg, resultsChannel)
	}

	go func() {
		for v := range resultsChannel {
			results = append(results, v)
		}
	}()

	wg.Wait()
	close(resultsChannel)

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].deviceID < results[j].deviceID
	})

	fmt.Printf("\nOutput\n")
	for r := range results {
		scanner := bufio.NewScanner(strings.NewReader(results[r].output))
		for scanner.Scan() {
			fmt.Printf("%s %s %s %s %s\n", results[r].device, separator, results[r].command, separator, scanner.Text())
		}
	}
}

func execCommands(deviceID int, user string, password string, device string, commands []string, wg *sync.WaitGroup, resultsChannel chan output) {
	defer wg.Done()

	client, err := connectToDevice(user, password, device)
	if err != nil {
		for c := 0; c < len(commands); c++ {
			output := output{
				deviceID: deviceID,
				device:   device,
				command:  commands[c],
				output:   "error connecting to device",
			}
			resultsChannel <- output
		}
		log.Printf("error connecting to device %s", device)
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
				output:   "error creating session",
			}
			resultsChannel <- output
			log.Printf("error creating session for command %s on device %s", commands[c], device)
			session.Close()
			continue
		}

		out, err := session.CombinedOutput(commands[c])
		if err != nil {
			output := output{
				deviceID: deviceID,
				device:   device,
				command:  commands[c],
				output:   "error executing command",
			}
			resultsChannel <- output
			log.Printf("error executing command %s on device %s", commands[c], device)
			session.Close()
			continue
		}

		output := output{
			deviceID: deviceID,
			device:   device,
			command:  commands[c],
			output:   string(out),
		}

		resultsChannel <- output
		session.Close()
	}
}

func connectToDevice(user, password, device string) (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password(password)},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

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
