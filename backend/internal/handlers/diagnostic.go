package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

type DiagnosticHandler struct{}

func NewDiagnosticHandler() *DiagnosticHandler {
	return &DiagnosticHandler{}
}

// Ping runs ping from a MikroTik NAS to a target IP
func (h *DiagnosticHandler) Ping(c *fiber.Ctx) error {
	var req struct {
		NasID  uint   `json:"nas_id"`
		Target string `json:"target"`
		Size   int    `json:"size"`
		Count  int    `json:"count"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.NasID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "NAS ID is required"})
	}
	if req.Target == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Target IP is required"})
	}

	if req.Count <= 0 {
		req.Count = 50
	}
	if req.Count > 100 {
		req.Count = 100
	}

	// Load NAS
	var nas models.Nas
	if err := database.DB.First(&nas, req.NasID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "NAS not found"})
	}

	// Connect to MikroTik
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	pingResult, err := client.Ping(req.Target, req.Count, req.Size)

	// Format output
	var output strings.Builder
	sizeLabel := 32
	if req.Size > 0 {
		sizeLabel = req.Size
	}
	output.WriteString(fmt.Sprintf("\nPinging %s via %s (size=%d):\n\n", req.Target, nas.Name, sizeLabel))

	if err != nil {
		output.WriteString(fmt.Sprintf("Ping failed: %v\n", err))
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"output":  output.String(),
				"result":  nil,
				"ping_ok": false,
			},
		})
	}

	// Show individual packet replies with real RTT per packet
	if len(pingResult.RTTs) > 0 {
		for i, rtt := range pingResult.RTTs {
			output.WriteString(fmt.Sprintf("  seq=%d  Reply from %s: bytes=%d time=%.2fms TTL=64\n", i, req.Target, sizeLabel, rtt))
		}
		lost := pingResult.Sent - pingResult.Received
		for i := 0; i < lost; i++ {
			output.WriteString("  Request timed out.\n")
		}
	} else {
		for i := 0; i < pingResult.Sent; i++ {
			output.WriteString("  Request timed out.\n")
		}
	}

	output.WriteString(fmt.Sprintf("\nPing statistics for %s:\n", req.Target))
	output.WriteString(fmt.Sprintf("    Packets: Sent = %d, Received = %d, Lost = %d (%d%% loss)\n",
		pingResult.Sent, pingResult.Received, pingResult.Sent-pingResult.Received, pingResult.PacketLoss))

	if pingResult.Received > 0 {
		output.WriteString("Approximate round trip times in milli-seconds:\n")
		output.WriteString(fmt.Sprintf("    Minimum = %.1fms, Maximum = %.1fms, Average = %.1fms\n",
			pingResult.MinRTT, pingResult.MaxRTT, pingResult.AvgRTT))
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"output":  output.String(),
			"result":  pingResult,
			"ping_ok": pingResult.Received > 0,
		},
	})
}

// PingStream runs ping with live streaming results (NDJSON)
func (h *DiagnosticHandler) PingStream(c *fiber.Ctx) error {
	var req struct {
		NasID  uint   `json:"nas_id"`
		Target string `json:"target"`
		Size   int    `json:"size"`
		Count  int    `json:"count"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.NasID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "NAS ID is required"})
	}
	if req.Target == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Target IP is required"})
	}
	if req.Count <= 0 {
		req.Count = 50
	}
	if req.Count > 100 {
		req.Count = 100
	}

	var nas models.Nas
	if err := database.DB.First(&nas, req.NasID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "NAS not found"})
	}

	sizeLabel := 32
	if req.Size > 0 {
		sizeLabel = req.Size
	}

	c.Set("Content-Type", "application/x-ndjson")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")
	c.Set("X-Content-Type-Options", "nosniff")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		enc := json.NewEncoder(w)

		client := mikrotik.NewClient(
			fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
			nas.APIUsername,
			nas.APIPassword,
		)
		defer client.Close()

		// Send start event
		enc.Encode(map[string]interface{}{
			"type":   "start",
			"target": req.Target,
			"nas":    nas.Name,
			"size":   sizeLabel,
			"count":  req.Count,
		})
		w.Flush()

		result, err := client.PingLive(req.Target, req.Count, req.Size, func(event mikrotik.PingEvent) {
			enc.Encode(event)
			w.Flush()
		})

		if err != nil {
			enc.Encode(map[string]interface{}{
				"type":    "error",
				"message": err.Error(),
			})
			w.Flush()
			return
		}

		// Send stats
		enc.Encode(map[string]interface{}{
			"type":     "stats",
			"target":   req.Target,
			"sent":     result.Sent,
			"received": result.Received,
			"lost":     result.Sent - result.Received,
			"loss":     result.PacketLoss,
			"min":      result.MinRTT,
			"max":      result.MaxRTT,
			"avg":      result.AvgRTT,
		})
		w.Flush()
	})

	return nil
}

// isPrivateIP checks if the target is a private/reserved IP
func isPrivateIP(target string) bool {
	ip := net.ParseIP(target)
	if ip == nil {
		return false // hostname, not IP - allow it
	}
	privateRanges := []struct {
		network string
		mask    int
	}{
		{"10.0.0.0", 8},
		{"172.16.0.0", 12},
		{"192.168.0.0", 16},
		{"127.0.0.0", 8},
	}
	for _, r := range privateRanges {
		_, cidr, _ := net.ParseCIDR(fmt.Sprintf("%s/%d", r.network, r.mask))
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// Traceroute runs traceroute from the server to a public target
func (h *DiagnosticHandler) Traceroute(c *fiber.Ctx) error {
	var req struct {
		Target string `json:"target"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.Target == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Target is required"})
	}

	// Block private IPs
	if isPrivateIP(req.Target) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Only public IPs/hostnames are supported for traceroute"})
	}

	// Validate target - only allow alphanumeric, dots, hyphens, colons (IPv6)
	validTarget := regexp.MustCompile(`^[a-zA-Z0-9.\-:]+$`)
	if !validTarget.MatchString(req.Target) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid target format"})
	}

	// Run traceroute from server with 30-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "traceroute", "-n", "-m", "30", "-w", "2", req.Target)
	output, err := cmd.CombinedOutput()

	type TracerouteHop struct {
		Hop     int     `json:"hop"`
		Address string  `json:"address"`
		Loss    string  `json:"loss"`
		Last    float64 `json:"last"`
		Avg     float64 `json:"avg"`
		Best    float64 `json:"best"`
		Worst   float64 `json:"worst"`
	}

	var hops []TracerouteHop

	if len(output) > 0 {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "traceroute to") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			hopNum, parseErr := strconv.Atoi(fields[0])
			if parseErr != nil {
				continue
			}

			hop := TracerouteHop{Hop: hopNum}

			if fields[1] == "*" {
				hop.Address = "*"
				hop.Loss = "100%"
			} else {
				hop.Address = fields[1]
				hop.Loss = "0%"
				// Parse RTT values (e.g., "1.234 ms")
				var rtts []float64
				for i := 2; i < len(fields); i++ {
					if fields[i] == "ms" || fields[i] == "*" {
						continue
					}
					if rtt, e := strconv.ParseFloat(fields[i], 64); e == nil {
						rtts = append(rtts, rtt)
					}
				}
				if len(rtts) > 0 {
					hop.Last = rtts[len(rtts)-1]
					hop.Best = rtts[0]
					hop.Worst = rtts[0]
					sum := 0.0
					for _, r := range rtts {
						sum += r
						if r < hop.Best {
							hop.Best = r
						}
						if r > hop.Worst {
							hop.Worst = r
						}
					}
					hop.Avg = sum / float64(len(rtts))
				}
			}
			hops = append(hops, hop)
		}
	}

	if err != nil && len(hops) == 0 {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"hops":   []interface{}{},
				"target": req.Target,
				"source": "Server",
				"error":  fmt.Sprintf("Traceroute failed: %v", err),
			},
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"hops":   hops,
			"target": req.Target,
			"source": "Server",
		},
	})
}

// NSLookup performs DNS lookups from the server
func (h *DiagnosticHandler) NSLookup(c *fiber.Ctx) error {
	var req struct {
		Domain string `json:"domain"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.Domain == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Domain is required"})
	}

	// Clean domain
	domain := strings.TrimSpace(req.Domain)
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.Split(domain, "/")[0]

	result := fiber.Map{}

	// A/AAAA records
	ips, err := net.LookupHost(domain)
	if err != nil {
		log.Printf("NSLookup: LookupHost failed for %s: %v", domain, err)
	} else {
		var aRecords []string
		var aaaaRecords []string
		for _, ip := range ips {
			if strings.Contains(ip, ":") {
				aaaaRecords = append(aaaaRecords, ip)
			} else {
				aRecords = append(aRecords, ip)
			}
		}
		result["a"] = aRecords
		result["aaaa"] = aaaaRecords
	}

	// CNAME
	cname, err := net.LookupCNAME(domain)
	if err == nil && cname != "" && cname != domain+"." {
		result["cname"] = cname
	}

	// MX records
	mxRecords, err := net.LookupMX(domain)
	if err == nil {
		var mxList []fiber.Map
		for _, mx := range mxRecords {
			mxList = append(mxList, fiber.Map{
				"host":     mx.Host,
				"priority": mx.Pref,
			})
		}
		result["mx"] = mxList
	}

	// NS records
	nsRecords, err := net.LookupNS(domain)
	if err == nil {
		var nsList []string
		for _, ns := range nsRecords {
			nsList = append(nsList, ns.Host)
		}
		result["ns"] = nsList
	}

	// TXT records
	txtRecords, err := net.LookupTXT(domain)
	if err == nil {
		result["txt"] = txtRecords
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"domain":  domain,
			"records": result,
		},
	})
}

// SearchSubscribers searches for subscribers by username or IP for autocomplete
func (h *DiagnosticHandler) SearchSubscribers(c *fiber.Ctx) error {
	nasID := c.QueryInt("nas_id", 0)
	query := c.Query("q", "")

	if query == "" || len(query) < 2 {
		return c.JSON(fiber.Map{"success": true, "data": []interface{}{}})
	}

	type SubscriberResult struct {
		ID        uint   `json:"id"`
		Username  string `json:"username"`
		IPAddress string `json:"ip_address"`
		StaticIP  string `json:"static_ip"`
		IsOnline  bool   `json:"is_online"`
	}

	var results []SubscriberResult

	dbQuery := database.DB.Model(&models.Subscriber{}).
		Select("id, username, ip_address, is_online").
		Where("deleted_at IS NULL").
		Where("(username ILIKE ? OR ip_address ILIKE ?)", "%"+query+"%", "%"+query+"%").
		Order("username ASC").
		Limit(10)

	if nasID > 0 {
		dbQuery = dbQuery.Where("nas_id = ?", nasID)
	}

	if err := dbQuery.Find(&results).Error; err != nil {
		log.Printf("DiagnosticSearchSubscribers: query failed: %v", err)
		return c.JSON(fiber.Map{"success": true, "data": []interface{}{}})
	}

	// Also fetch static IPs from radreply
	for i := range results {
		var staticIP string
		database.DB.Model(&models.RadReply{}).
			Where("username = ? AND attribute = ?", results[i].Username, "Framed-IP-Address").
			Pluck("value", &staticIP)
		results[i].StaticIP = staticIP
	}

	return c.JSON(fiber.Map{"success": true, "data": results})
}
