// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"camel/lizard"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/leesper/holmes"
	"github.com/spf13/cobra"
)

type GatewayServer struct {
	*lizard.Server
}

// gatewayCmd represents the gateway command
var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "A brief description of your command",
	Long:  `A longer description.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("gateway called")
		addr := cmd.Flags().Lookup("address").Value.String()
		defer holmes.Start().Stop()

		lizard.Register(chat.ChatMessage, chat.DeserializeMessage, chat.ProcessMessage)

		l, err := net.Listen("tcp", addr)
		if err != nil {
			holmes.Fatalln("listen error", err)
		}
		chatServer := NewGatewayServer()

		go func() {
			c := make(chan os.Signal, 1)
			signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
			<-c
			chatServer.Stop()
		}()

		holmes.Infoln(chatServer.Start(l))
	},
}

func init() {
	rootCmd.AddCommand(gatewayCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// gatewayCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	gatewayCmd.Flags().StringP("address", "a", "localhost:9090", "gateway server address")
	gatewayCmd.Flags().StringP("web", "w", "localhost:9092", "web server address")
}

// NewGatewayServer returns a ChatServer.
func NewGatewayServer() *GatewayServer {
	onConnectOption := lizard.OnConnectOption(func(conn lizard.WriteCloser) bool {
		holmes.Infoln("on connect")
		return true
	})
	onErrorOption := lizard.OnErrorOption(func(conn lizard.WriteCloser) {
		holmes.Infoln("on error")
	})
	onCloseOption := lizard.OnCloseOption(func(conn lizard.WriteCloser) {
		holmes.Infoln("close chat client")
	})
	return &GatewayServer{
		lizard.NewServer(onConnectOption, onErrorOption, onCloseOption),
	}
}
