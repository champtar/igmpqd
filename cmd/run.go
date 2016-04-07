// Copyright © 2016 John Mylchreest <jmylchreest@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the IGMPQD daemon",
	Long: `
`,
	Run: func(cmd *cobra.Command, args []string) {
		debug(fmt.Sprintf("IGMPQD Started."))
		debug(fmt.Sprintf("%15s: %s", "version", GitDescribe))
		debug(fmt.Sprintf("%15s: %s", "commit", GitCommit))
		debug(fmt.Sprintf("%15s: %s", "grpAddress", viper.GetString("grpAddress")))
		debug(fmt.Sprintf("%15s: %s", "dstAddress", viper.GetString("dstAddress")))
		debug(fmt.Sprintf("%15s: %d", "maxResponseTime", viper.GetInt("maxResponseTime")))
		debug(fmt.Sprintf("%15s: %d", "interval", viper.GetInt("interval")))

		tickC := time.NewTicker(time.Second * time.Duration(viper.GetInt("interval"))).C
		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, os.Interrupt)
		go func() {
			sendPacket()
			for _ = range tickC {
				sendPacket()
			}
		}()
		<-signalC
	},
}

func sendPacket() {
	var payload []byte

	timeoutDuration, _ := time.ParseDuration("10s")
	grpAddress := net.ParseIP(viper.GetString("grpAddress"))
	dstAddress := net.ParseIP(viper.GetString("dstAddress"))

	// IGMP https://tools.ietf.org/html/rfc2236#section-2
	payload = make([]byte, 8, 8)
	payload[0] = uint8(0x11)
	payload[1] = uint8(viper.GetInt("maxResponseTime"))
	payload[4] = grpAddress.To4()[0]
	payload[5] = grpAddress.To4()[1]
	payload[6] = grpAddress.To4()[2]
	payload[7] = grpAddress.To4()[3]
	binary.BigEndian.PutUint16(payload[2:], genChecksum(payload, 0))

	// Send packet
	debug("opening socket.")
	conn, err := net.DialTimeout("ip:igmp", dstAddress.String(), timeoutDuration)
	if err != nil {
		log.Fatal("Error occured. ", err)
	}

	debug(fmt.Sprintf("sending payload: %d", payload))
	_, err = conn.Write(payload)
	if err != nil {
		log.Fatal("Error occured. ", err)
	}

	debug("closing socket.")
	conn.Close()
}

func init() {
	RootCmd.AddCommand(runCmd)

	runCmd.PersistentFlags().Bool("debug", false, "Enable debug messages to stderr.")
	runCmd.PersistentFlags().StringP("grpAddress", "g", "0.0.0.0", "Specified IP address to use as the Group Address. Used to query for specific group members.")
	runCmd.PersistentFlags().StringP("dstAddress", "d", "224.0.0.1", "Specified IP address to send the IGMP Query to. (Defaults to: 224.0.0.1)")
	runCmd.PersistentFlags().IntP("interval", "i", 30, "The time in seconds to delay between sending IGMP Query messages. (Defaults to: 30)")
	runCmd.PersistentFlags().IntP("maxResponseTime", "m", 100, "Specifies the maximum allowed time before sending a responding report in units of 1/10 second. (Defaults to: 100)")

	viper.BindPFlag("debug", runCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("grpAddress", runCmd.PersistentFlags().Lookup("grpAddress"))
	viper.BindPFlag("dstAddress", runCmd.PersistentFlags().Lookup("dstAddress"))
	viper.BindPFlag("interval", runCmd.PersistentFlags().Lookup("interval"))
	viper.BindPFlag("maxResponseTime", runCmd.PersistentFlags().Lookup("maxResponseTime"))
}

func debug(message string) {
	if viper.GetBool("debug") {
		log.Println(message)
	}
}

// Calculate the TCP/IP checksum defined in rfc1071.
func genChecksum(data []byte, csum uint32) uint16 {
	length := len(data) - 1
	for i := 0; i < length; i += 2 {
		csum += uint32(data[i]) << 8
		csum += uint32(data[i+1])
	}
	if len(data)%2 == 1 {
		csum += uint32(data[length]) << 8
	}
	for csum > 0xffff {
		csum = (csum & 0xffff) + (csum >> 16)
	}
	return ^uint16((csum >> 16) + csum)
}
