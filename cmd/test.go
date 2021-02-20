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

	//kh "golang.org/x/crypto/ssh/knownhosts"
	kh "github.com/cburnette/gather/knownhostspatched"
)

type device struct {
	deviceID int
	device   string
	outputs  []output
}

type output struct {
	command string
	output  string
}

var mutex = &sync.Mutex{}
var results []device
var user string
var password []byte

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
	fmt.Printf("Output File: ")
	fmt.Printf("%s\n\n", outputFile)

	var hostsWhitelist []string
	devices := getDevices()
	fmt.Printf("Devices:\n")
	for _, device := range devices {
		deviceWithoutPort := strings.Split(device.device, ":")[0]
		hostsWhitelist = append(hostsWhitelist, deviceWithoutPort)
		fmt.Println(device.device)
	}

	commands := getCommands()
	fmt.Printf("\nCommands:\n")
	for _, command := range commands {
		fmt.Println(command)
	}
	//fmt.Println()

	fmt.Print("\nuser: ")
	fmt.Scanf("%s", &user)

	fmt.Print("password: ")
	password, err = terminal.ReadPassword(0)
	if err != nil {
		panic(err)
	}
	fmt.Println()
	fmt.Println()

	// user = "test"
	// password = []byte("test")

	var wg sync.WaitGroup
	wg.Add(len(devices))

	debug, err := rootCmd.PersistentFlags().GetBool("debug")
	if err != nil {
		panic(err)
	}

	sshConfig := buildSSHConfig(hostsWhitelist)
	fmt.Println()

	for _, device := range devices {
		if !debug {
			go execCommands(device, sshConfig, commands, &wg)
		} else {
			execCommands(device, sshConfig, commands, &wg)
		}
	}

	if !debug {
		for len(results) < len(devices) {
			wg.Wait()
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].deviceID < results[j].deviceID
	})

	separator, err := rootCmd.PersistentFlags().GetString("separator")
	if err != nil {
		panic(err)
	}
	writeOutputFile(results, outputFile, separator)
}

func addResult(d device) {
	mutex.Lock()
	results = append(results, d)
	mutex.Unlock()
}

func writeOutputFile(results []device, outputFile string, separator string) {
	f, err := os.Create(outputFile)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	writer := bufio.NewWriter(f)

	for _, device := range results {
		for _, output := range device.outputs {
			scanner := bufio.NewScanner(strings.NewReader(output.output))
			for scanner.Scan() {
				_, err = writer.WriteString(fmt.Sprintf("%s %s %s %s %s\n", device.device, separator, output.command, separator, scanner.Text()))
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}

	writer.Flush()
}

func execCommands(device device, sshConfig *ssh.ClientConfig, commands []string, wg *sync.WaitGroup) {
	defer wg.Done()

	client, err := connectToDevice(device, sshConfig)
	if err != nil {
		for c := 0; c < len(commands); c++ {
			output := output{
				command: commands[c],
				output:  err.Error(),
			}
			device.outputs = append(device.outputs, output)
		}
		addResult(device)
		//log.Printf(err.Error())
		return
	}

	defer client.Close()

	for _, command := range commands {
		session, err := client.NewSession()

		if err != nil {
			output := output{
				command: command,
				output:  err.Error(),
			}
			device.outputs = append(device.outputs, output)
			//log.Printf("%s %s %s %s %s\n", device.device, separator, command, separator, err.Error())
			session.Close()
			continue
		}

		var b bytes.Buffer
		session.Stdout = &b
		if err := session.Run(command); err != nil {
			output := output{
				command: command,
				output:  err.Error(),
			}
			device.outputs = append(device.outputs, output)
			//log.Printf("%s %s %s %s %s\n", device.device, separator, command, separator, err.Error())
			session.Close()
			continue
		}

		output := output{
			command: command,
			output:  b.String(),
		}

		device.outputs = append(device.outputs, output)
		session.Close()
	}
	addResult(device)
}

func buildSSHConfig(hostsWhitelist []string) *ssh.ClientConfig {
	insecure, err := rootCmd.PersistentFlags().GetBool("insecure")
	if err != nil {
		panic(err)
	}

	var sshConfig *ssh.ClientConfig

	if insecure {
		sshConfig = &ssh.ClientConfig{
			User:            user,
			Auth:            []ssh.AuthMethod{ssh.Password(string(password))},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         5 * time.Second,
		}
	} else {
		knownHostsFile, err := rootCmd.PersistentFlags().GetString("knownHosts")
		if err != nil {
			panic(err)
		}

		if knownHostsFile == defaultKnownHostsFile {
			home, err := os.UserHomeDir()
			if err != nil {
				panic(err)
			}

			knownHostsFile = fmt.Sprintf("%s/.ssh/known_hosts", home)
		}

		hostKeyCallback, err := kh.New(hostsWhitelist, knownHostsFile)
		if err != nil {
			panic(err)
		}

		sshConfig = &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(string(password)),
				ssh.KeyboardInteractive(sshInteractive),
			},
			HostKeyCallback: hostKeyCallback,
			Timeout:         5 * time.Second,
		}
	}

	return sshConfig
}

func sshInteractive(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
	answers = make([]string, len(questions))

	for n := range questions {
		answers[n] = string(password)
	}

	return answers, nil
}

func connectToDevice(device device, sshConfig *ssh.ClientConfig) (*ssh.Client, error) {

	client, err := ssh.Dial("tcp", device.device, sshConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func getDevices() []device {
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
	var devices []device

	i := 0
	for scanner.Scan() {
		deviceName := scanner.Text()
		deviceName = strings.TrimSpace(deviceName)

		if !strings.HasPrefix(deviceName, "#") {
			parts := strings.Split(deviceName, ":")
			if len(parts) == 1 {
				deviceName = deviceName + ":22"
			}

			newDevice := device{
				deviceID: i,
				device:   deviceName,
				outputs:  []output{},
			}
			devices = append(devices, newDevice)
		}
		i++
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
