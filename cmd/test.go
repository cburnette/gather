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
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

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
	fmt.Println("Starting test")

	startPort, err := rootCmd.PersistentFlags().GetString("startPort")
	if err != nil {
		panic(err)
	}

	host, err := rootCmd.PersistentFlags().GetString("host")
	if err != nil {
		panic(err)
	}

	client, session, err := connectToHost("test", host+":"+startPort)
	if err != nil {
		panic(err)
	}
	out, err := session.CombinedOutput("hostname -I | awk '{print $1}'")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(out))
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

// osCmd := exec.Command("tr", "a-z", "A-Z")
// osCmd.Stdin = strings.NewReader("some input")
// var out bytes.Buffer
// osCmd.Stdout = &out
// err := osCmd.Run()
// if err != nil {
// 	log.Fatal(err)
// }
// fmt.Printf("in all caps: %q\n", out.String())
