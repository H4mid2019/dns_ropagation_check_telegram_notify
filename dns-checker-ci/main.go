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

// --- telegramMessage struct and sendTelegramMessage function remain the same ---
type telegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func sendTelegramMessage(botToken, chatID, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	message := telegramMessage{
		ChatID: chatID, Text: text, ParseMode: "Markdown",
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

func main() {
	// --- Read configuration ---
	domain := flag.String("domain", "", "The domain name to check")
	dnsServer := flag.String("dns", "1.1.1.1:53", "The DNS server to use (host:port)")
	flag.Parse()

	if *domain == "" {
		log.Fatal("Error: The --domain flag is required.")
		os.Exit(1)
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" || chatID == "" {
		log.Fatal("Error: TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must be set.")
		os.Exit(1)
	}

	if dnsServer == nil || *dnsServer == "" {
		*dnsServer = "1.1.1.1:53"
	}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}
			return d.DialContext(ctx, "udp", *dnsServer)
		},
	}

	log.Printf("Performing a single DNS check for %s...", *domain)

	// --- Check for A records ---
	ips, err := resolver.LookupIP(context.Background(), "ip4", *domain)
	if err == nil && len(ips) > 0 {
		log.Println("Found A records. Sending notification.")
		messageText := fmt.Sprintf("✅ *A records found for %s*\n", *domain)
		for _, ip := range ips {
			messageText += fmt.Sprintf("  - `%s`\n", ip.String())
		}
		if err := sendTelegramMessage(botToken, chatID, messageText); err != nil {
			log.Printf("Error sending Telegram notification: %v", err)
		}
	} else {
		log.Println("A records not found.")
	}

	// --- Check for NS records ---
	ns, err := resolver.LookupNS(context.Background(), *domain)
	if err == nil && len(ns) > 0 {
		log.Println("Found NS records. Sending notification.")
		messageText := fmt.Sprintf("✅ *NS records found for %s*\n", *domain)
		for _, n := range ns {
			messageText += fmt.Sprintf("  - `%s`\n", n.Host)
		}
		if err := sendTelegramMessage(botToken, chatID, messageText); err != nil {
			log.Printf("Error sending Telegram notification: %v", err)
		}
	} else {
		log.Println("NS records not found.")
	}

	// --- Check for MX records ---
	mx, err := resolver.LookupMX(context.Background(), *domain)
	if err == nil && len(mx) > 0 {
		log.Println("Found MX records. Sending notification.")
		messageText := fmt.Sprintf("✅ *MX records found for %s*\n", *domain)
		for _, m := range mx {
			messageText += fmt.Sprintf("  - Host: `%s`, Pref: %d\n", m.Host, m.Pref)
		}
		if err := sendTelegramMessage(botToken, chatID, messageText); err != nil {
			log.Printf("Error sending Telegram notification: %v", err)
		}
	} else {
		log.Println("MX records not found.")
	}

	log.Println("DNS check complete. Exiting.")
}
