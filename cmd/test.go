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
)

type output struct {
	targetID int
	host     string
	command  string
	output   string
}

func (o output) String() string {
	separator, err := rootCmd.PersistentFlags().GetString("separator")
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("%s %s %s %s %s", o.host, separator, o.command, separator, o.output)
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
	targets := getTargets()
	fmt.Printf("Targets:\n")
	for _, target := range targets {
		fmt.Println(target)
	}

	commands := getCommands()
	fmt.Printf("\nCommands:\n")
	for _, command := range commands {
		fmt.Println(command)
	}

	var results []output
	resultsChannel := make(chan output)
	var wg sync.WaitGroup

	for t := 0; t < len(targets); t++ {
		wg.Add(1)
		go execCommands(t, "test", targets[t], commands, &wg, resultsChannel)
	}

	go func() {
		for v := range resultsChannel {
			results = append(results, v)
		}
	}()

	wg.Wait()
	close(resultsChannel)

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].targetID < results[j].targetID
	})

	fmt.Printf("\nOutput\n")
	for r := range results {
		fmt.Print(results[r])
	}
}

func execCommands(targetID int, user string, host string, commands []string, wg *sync.WaitGroup, resultsChannel chan output) {
	defer wg.Done()

	client, err := connectToHost(user, host)
	if err != nil {
		log.Printf("error connecting to device %s", host)
	}
	defer client.Close()

	for c := 0; c < len(commands); c++ {
		session, err := client.NewSession()
		if err != nil {
			output := output{
				targetID: targetID,
				host:     host,
				command:  commands[c],
				output:   "error creating session",
			}
			resultsChannel <- output
			log.Printf("error creating session for command %s on device %s", commands[c], host)
			continue
		}

		out, err := session.CombinedOutput(commands[c])
		if err != nil {
			output := output{
				targetID: targetID,
				host:     host,
				command:  commands[c],
				output:   "error executing command",
			}
			resultsChannel <- output
			log.Printf("error executing command %s on device %s", commands[c], host)
			continue
		}

		output := output{
			targetID: targetID,
			host:     host,
			command:  commands[c],
			output:   string(out),
		}

		resultsChannel <- output
	}
}

func connectToHost(user, host string) (*ssh.Client, error) {
	pass := "test"

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password(pass)},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func getTargets() []string {
	targetFile, err := rootCmd.PersistentFlags().GetString("targets")
	if err != nil {
		panic(err)
	}

	file, err := os.Open(targetFile)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var targets []string

	for scanner.Scan() {
		target := scanner.Text()
		if !strings.HasPrefix(target, "#") {
			parts := strings.Split(target, ":")
			if len(parts) == 1 {
				target = target + ":22"
			}
			targets = append(targets, target)
		}
	}

	return targets
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
