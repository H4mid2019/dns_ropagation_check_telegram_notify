package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

// Represents the payload for the Telegram sendMessage API call.
type telegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"` // Enables Markdown formatting
}

// sendTelegramMessage sends a message via the Telegram Bot API.
func sendTelegramMessage(botToken, chatID, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	message := telegramMessage{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned non-200 status: %s", resp.Status)
	}

	return nil
}

// Keeps track of which DNS records have been found.
type discoveryState struct {
	aRecordFound  bool
	nsRecordFound bool
	mxRecordFound bool
}

func main() {
	// --- Command-line flag for the domain ---
	domain := flag.String("domain", "", "The domain name to check (e.g., google.com)")
	dnsServer := flag.String("dns", "1.1.1.1:53", "The DNS server to use (host:port)")
	interval := flag.Int("interval", 60, "Interval in minutes between checks")
	flag.Parse()

	// Exit if the domain flag is not provided.
	if *domain == "" {
		log.Println("Error: The --domain flag is required.")
		os.Exit(1) // Exits with a non-zero status code.
	}

	// --- Configuration ---
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if dnsServer == nil || *dnsServer == "" {
		*dnsServer = "1.1.1.1:53"
	}
	if interval == nil || *interval <= 0 {
		*interval = 60 // Default to 60 minutes if invalid
	}
	checkInterval := time.Duration(*interval) * time.Minute

	// Custom resolver to use the specified DNS server.
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}
			return d.DialContext(ctx, "udp", *dnsServer)
		},
	}

	state := &discoveryState{}
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	log.Printf("Starting DNS propagation check for %s. Will check every %v.", *domain, checkInterval)

	// --- Main polling loop ---
	for {
		if state.aRecordFound && state.nsRecordFound && state.mxRecordFound {
			log.Println("All DNS records found. Exiting.")
			finalMessage := fmt.Sprintf("✅ *All records found for %s*!", *domain)
			sendTelegramMessage(botToken, chatID, finalMessage) // Optional final notification
			break
		}

		// Check for A records if not already found
		if !state.aRecordFound {
			log.Printf("Checking A records for %s...", *domain)
			ips, err := resolver.LookupIP(context.Background(), "ip4", *domain)
			if err == nil && len(ips) > 0 {
				log.Println("Found A records! Sending notification.")
				messageText := fmt.Sprintf("✅ *A records found for %s*\n", *domain)
				for _, ip := range ips {
					messageText += fmt.Sprintf("  - `%s`\n", ip.String())
				}
				if err := sendTelegramMessage(botToken, chatID, messageText); err != nil {
					log.Printf("Error sending Telegram notification: %v", err)
				}
				state.aRecordFound = true
			}
		}

		// ... (NS and MX record checks follow the same pattern) ...
		// Check for NS records if not already found
		if !state.nsRecordFound {
			log.Printf("Checking NS records for %s...", *domain)
			ns, err := resolver.LookupNS(context.Background(), *domain)
			if err == nil && len(ns) > 0 {
				log.Println("Found NS records! Sending notification.")
				messageText := fmt.Sprintf("✅ *NS records found for %s*\n", *domain)
				for _, n := range ns {
					messageText += fmt.Sprintf("  - `%s`\n", n.Host)
				}
				if err := sendTelegramMessage(botToken, chatID, messageText); err != nil {
					log.Printf("Error sending Telegram notification: %v", err)
				}
				state.nsRecordFound = true
			}
		}

		// Check for MX records if not already found
		if !state.mxRecordFound {
			log.Printf("Checking MX records for %s...", *domain)
			mx, err := resolver.LookupMX(context.Background(), *domain)
			if err == nil && len(mx) > 0 {
				log.Println("Found MX records! Sending notification.")
				messageText := fmt.Sprintf("✅ *MX records found for %s*\n", *domain)
				for _, m := range mx {
					messageText += fmt.Sprintf("  - Host: `%s`, Pref: %d\n", m.Host, m.Pref)
				}
				if err := sendTelegramMessage(botToken, chatID, messageText); err != nil {
					log.Printf("Error sending Telegram notification: %v", err)
				}
				state.mxRecordFound = true
			}
		}

		<-ticker.C
	}
}
