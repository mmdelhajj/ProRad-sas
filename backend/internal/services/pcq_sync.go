package services

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

// parseCDNSubnets parses a subnet string that may be comma or newline separated
func parseCDNSubnets(subnetsStr string) []string {
	var subnets []string
	if subnetsStr == "" {
		return subnets
	}
	// Replace newlines with commas, then split by comma
	subnetsStr = strings.ReplaceAll(subnetsStr, "\r\n", ",")
	subnetsStr = strings.ReplaceAll(subnetsStr, "\n", ",")
	subnetsStr = strings.ReplaceAll(subnetsStr, "\r", ",")
	for _, subnet := range strings.Split(subnetsStr, ",") {
		subnet = strings.TrimSpace(subnet)
		if subnet != "" {
			subnets = append(subnets, subnet)
		}
	}
	return subnets
}

// SyncAllPCQOnStartup syncs all PCQ configurations to NAS devices when the API starts
// This ensures CDN queues are ready before any users connect
func SyncAllPCQOnStartup() {
	// Wait a few seconds for database connections to stabilize
	time.Sleep(5 * time.Second)

	log.Println("PCQ Startup Sync: Starting PCQ synchronization to all NAS devices...")

	// Get company name for branding
	companyName := getPCQCompanyName()

	// Find all ServiceCDNs with PCQ enabled
	var serviceCDNs []models.ServiceCDN
	database.DB.Preload("CDN").Preload("Service").Where("pcq_enabled = ? AND is_active = ?", true, true).Find(&serviceCDNs)

	if len(serviceCDNs) == 0 {
		log.Println("PCQ Startup Sync: No PCQ-enabled CDN configurations found")
		return
	}

	// Track which CDN+NAS+Speed combinations we've synced (each speed needs its own queue)
	synced := make(map[string]bool)
	syncCount := 0

	for _, sc := range serviceCDNs {
		// Skip if no NAS or pools configured or CDN not loaded
		if sc.PCQNASID == nil || sc.PCQTargetPools == "" || sc.SpeedLimit == 0 || sc.CDN == nil {
			continue
		}

		// Create unique key for CDN+NAS+Speed combination (each speed gets own queue)
		key := fmt.Sprintf("%d-%d-%d", sc.CDNID, *sc.PCQNASID, sc.SpeedLimit)
		if synced[key] {
			continue
		}
		synced[key] = true

		// Get NAS
		var nas models.Nas
		if err := database.DB.First(&nas, *sc.PCQNASID).Error; err != nil {
			log.Printf("PCQ Startup Sync: NAS %d not found", *sc.PCQNASID)
			continue
		}

		if !nas.IsActive {
			log.Printf("PCQ Startup Sync: NAS %s is not active, skipping", nas.Name)
			continue
		}

		// Connect to NAS
		client := mikrotik.NewClient(
			fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
			nas.APIUsername,
			nas.APIPassword,
		)

		// Parse CDN subnets (may be comma or newline separated)
		subnets := parseCDNSubnets(sc.CDN.Subnets)

		serviceName := ""
		if sc.Service != nil {
			serviceName = sc.Service.Name
		}

		pcqConfig := mikrotik.PCQConfig{
			CDNName:       sc.CDN.Name,
			SpeedLimitM:   sc.SpeedLimit,
			PCQLimit:      sc.PCQLimit,
			PCQTotalLimit: sc.PCQTotalLimit,
			TargetPools:   sc.PCQTargetPools,
			CompanyName:   companyName,
			Subnets:       subnets,
			ServiceName:   serviceName,
		}

		if err := client.SyncCDNPCQSetup(pcqConfig); err != nil {
			log.Printf("PCQ Startup Sync: Failed to sync CDN %s to NAS %s: %v", sc.CDN.Name, nas.Name, err)
			client.Close()
			continue
		}

		client.Close()
		syncCount++
		log.Printf("PCQ Startup Sync: Synced CDN %s to NAS %s (target=%s)", sc.CDN.Name, nas.Name, sc.PCQTargetPools)
	}

	log.Printf("PCQ Startup Sync: Completed - synced %d PCQ configurations", syncCount)
}

// getPCQCompanyName retrieves company name from settings for PCQ sync
func getPCQCompanyName() string {
	name := database.GetCompanyName()
	if name == "" {
		return "ISP"
	}
	// Remove any quotes from the value
	return strings.Trim(name, "\"")
}

// StartPCQAutoRepairService starts a background service that checks and repairs PCQ configurations every 5 minutes
// This ensures PCQ rules, queues, and mangle rules are always in place even if someone manually deletes them
func StartPCQAutoRepairService() {
	// Wait for startup sync to complete first
	time.Sleep(30 * time.Second)

	log.Println("PCQ Auto-Repair Service: Starting (checks every 5 minutes)")

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		repairPCQConfigurations()
	}
}

// repairPCQConfigurations checks and re-syncs all PCQ configurations to MikroTik
func repairPCQConfigurations() {
	log.Println("PCQ Auto-Repair: Checking PCQ configurations...")

	companyName := getPCQCompanyName()

	// Find all ServiceCDNs with PCQ enabled
	var serviceCDNs []models.ServiceCDN
	database.DB.Preload("CDN").Preload("Service").Where("pcq_enabled = ? AND is_active = ?", true, true).Find(&serviceCDNs)

	if len(serviceCDNs) == 0 {
		return
	}

	// Track which CDN+NAS+Speed combinations we've checked (each speed needs its own queue)
	checked := make(map[string]bool)
	repairedCount := 0

	for _, sc := range serviceCDNs {
		// Skip if no NAS or pools configured or CDN not loaded
		if sc.PCQNASID == nil || sc.PCQTargetPools == "" || sc.SpeedLimit == 0 || sc.CDN == nil {
			continue
		}

		// Create unique key for CDN+NAS+Speed combination (each speed gets own queue)
		key := fmt.Sprintf("%d-%d-%d", sc.CDNID, *sc.PCQNASID, sc.SpeedLimit)
		if checked[key] {
			continue
		}
		checked[key] = true

		// Get NAS
		var nas models.Nas
		if err := database.DB.First(&nas, *sc.PCQNASID).Error; err != nil {
			continue
		}

		if !nas.IsActive {
			continue
		}

		// Connect to NAS
		client := mikrotik.NewClient(
			fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
			nas.APIUsername,
			nas.APIPassword,
		)

		// Parse CDN subnets (may be comma or newline separated)
		subnets := parseCDNSubnets(sc.CDN.Subnets)

		serviceName := ""
		if sc.Service != nil {
			serviceName = sc.Service.Name
		}

		pcqConfig := mikrotik.PCQConfig{
			CDNName:       sc.CDN.Name,
			SpeedLimitM:   sc.SpeedLimit,
			PCQLimit:      sc.PCQLimit,
			PCQTotalLimit: sc.PCQTotalLimit,
			TargetPools:   sc.PCQTargetPools,
			CompanyName:   companyName,
			Subnets:       subnets,
			ServiceName:   serviceName,
		}

		// SyncCDNPCQSetup will create or update - it handles missing components
		if err := client.SyncCDNPCQSetup(pcqConfig); err != nil {
			log.Printf("PCQ Auto-Repair: Failed to repair CDN %s on NAS %s: %v", sc.CDN.Name, nas.Name, err)
		} else {
			repairedCount++
		}

		client.Close()
	}

	// Also sync static IP protection
	syncStaticIPProtection()

	if repairedCount > 0 {
		log.Printf("PCQ Auto-Repair: Checked and synced %d PCQ configurations", repairedCount)
	}
}

// syncStaticIPProtection ensures static IP address list and protection scheduler are in place
func syncStaticIPProtection() {
	// Find all subscribers with static IPs
	var subscribers []models.Subscriber
	database.DB.Where("static_ip != '' AND static_ip IS NOT NULL AND nas_id IS NOT NULL").Find(&subscribers)

	if len(subscribers) == 0 {
		return
	}

	// Group by NAS
	nasSubs := make(map[uint][]models.Subscriber)
	for _, sub := range subscribers {
		if sub.NasID != nil {
			nasSubs[*sub.NasID] = append(nasSubs[*sub.NasID], sub)
		}
	}

	// Sync to each NAS
	for nasID, subs := range nasSubs {
		var nas models.Nas
		if err := database.DB.First(&nas, nasID).Error; err != nil || !nas.IsActive {
			continue
		}

		client := mikrotik.NewClient(
			fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
			nas.APIUsername,
			nas.APIPassword,
		)

		for _, sub := range subs {
			// AddStaticIPToAddressList also ensures the protection scheduler exists
			if err := client.AddStaticIPToAddressList(sub.StaticIP, sub.Username); err != nil {
				log.Printf("PCQ Auto-Repair: Failed to sync static IP %s for %s: %v", sub.StaticIP, sub.Username, err)
			}
		}

		client.Close()
	}
}
