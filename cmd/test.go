/*
Copyright © 2021 Hiarc <support@hiarcdb.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

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
		for c := 0; c < len(commands); c++ {
			wg.Add(1)
			go execCommand(t, "test", targets[t], commands[c], &wg, resultsChannel)
		}
	}

	go func() {
		for v := range resultsChannel {
			results = append(results, v)
		}
	}()

	wg.Wait()
	time.Sleep(1000)
	close(resultsChannel)

	fmt.Printf("\nOutput\n")
	for r := range results {
		fmt.Print(results[r])
	}
}

func execCommand(targetID int, user, host, command string, wg *sync.WaitGroup, resultsChannel chan output) {
	defer wg.Done()

	client, session, err := connectToHost(user, host)
	if err != nil {
		log.Fatal(err)
	}
	out, err := session.CombinedOutput(command)
	if err != nil {
		log.Fatal(err)
	}

	output := output{
		targetID: targetID,
		host:     host,
		command:  command,
		output:   string(out),
	}

	resultsChannel <- output
	client.Close()
}

func connectToHost(user, host string) (*ssh.Client, *ssh.Session, error) {
	pass := "test"

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password(pass)},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, session, nil
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
