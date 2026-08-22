package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/repository/sqlite"
	"github.com/spf13/cobra"
)

func init() {
	var nodeCmd = &cobra.Command{
		Use:   "node",
		Short: "Node management commands",
		Long:  `Manage and monitor proxy nodes (servers).`,
	}

	// node list
	var listAll bool
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List all nodes with real-time status",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runNodeList(store, listAll)
		},
	}
	listCmd.Flags().BoolVarP(&listAll, "all", "a", false, "Show all nodes including hidden ones")
	nodeCmd.AddCommand(listCmd)

	// node info <id>
	var infoCmd = &cobra.Command{
		Use:   "info <id>",
		Short: "Show detailed information for a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid node ID: %w", err)
			}
			return runNodeInfo(store, id)
		},
	}
	nodeCmd.AddCommand(infoCmd)

	// node stat <id>
	var statDays int
	var statRecordType int
	var statCmd = &cobra.Command{
		Use:   "stat <id>",
		Short: "Show historical statistics for a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid node ID: %w", err)
			}
			return runNodeStat(store, id, statRecordType, statDays)
		},
	}
	statCmd.Flags().IntVarP(&statDays, "days", "d", 7, "Number of days to show")
	statCmd.Flags().IntVarP(&statRecordType, "type", "t", 1, "Record type: 0=hourly, 1=daily")
	nodeCmd.AddCommand(statCmd)

	// node status
	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show real-time status of all online nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runNodeStatus(store)
		},
	}
	nodeCmd.AddCommand(statusCmd)

	rootCmd.AddCommand(nodeCmd)
}

func runNodeList(store *sqlite.Store, all bool) error {
	ctx := context.Background()
	var servers []*repository.Server
	var err error

	if all {
		servers, err = store.Servers().ListAll(ctx)
	} else {
		servers, err = store.Servers().FindAllVisible(ctx)
	}
	if err != nil {
		return err
	}

	if len(servers) == 0 {
		fmt.Println("No nodes found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tHOST:PORT\tSTATUS\tLAST SEEN")

	now := time.Now().Unix()
	for _, s := range servers {
		status := "●"
		statusColor := "\033[31m" // Red for offline
		if now-s.LastHeartbeatAt < 120 {
			statusColor = "\033[32m" // Green for online
		} else if now-s.LastHeartbeatAt < 300 {
			statusColor = "\033[33m" // Yellow for warning
		}

		lastSeen := "Never"
		if s.LastHeartbeatAt > 0 {
			lastSeen = formatDuration(now - s.LastHeartbeatAt)
		}

		hostPort := fmt.Sprintf("%s:%d", s.Host, s.Port)
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s%s\033[0m\t%s\n",
			s.ID, s.Name, s.Type, hostPort, statusColor, status, lastSeen)
	}
	if err := w.Flush(); err != nil {
		slog.Warn("failed to flush node list output", "error", err)
	}
	return nil
}

func runNodeInfo(store *sqlite.Store, id int64) error {
	ctx := context.Background()
	server, err := store.Servers().FindByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return fmt.Errorf("node with ID %d not found", id)
		}
		return err
	}

	now := time.Now().Unix()
	status := "Offline"
	if now-server.LastHeartbeatAt < 120 {
		status = "Online"
	} else if now-server.LastHeartbeatAt < 300 {
		status = "Warning"
	}

	fmt.Printf("Node Information (ID: %d)\n", server.ID)
	fmt.Println("========================================")
	fmt.Printf("Name:            %s\n", server.Name)
	fmt.Printf("Type:            %s\n", server.Type)
	fmt.Printf("Host:            %s\n", server.Host)
	fmt.Printf("Port:            %d\n", server.Port)
	fmt.Printf("Server Port:     %d\n", server.ServerPort)
	fmt.Printf("Code:            %s\n", server.Code)
	fmt.Printf("Group ID:        %d\n", server.GroupID)
	fmt.Printf("Rate:            %s\n", server.Rate)
	fmt.Printf("Status:          %s\n", status)
	fmt.Printf("Visible:         %v\n", server.Show == 1)
	fmt.Printf("Sort:            %d\n", server.Sort)

	if server.LastHeartbeatAt > 0 {
		lastSeen := time.Unix(server.LastHeartbeatAt, 0).Format("2006-01-02 15:04:05")
		fmt.Printf("Last Heartbeat:  %s (%s ago)\n", lastSeen, formatDuration(now-server.LastHeartbeatAt))
	} else {
		fmt.Printf("Last Heartbeat:  Never\n")
	}

	createdAt := time.Unix(server.CreatedAt, 0).Format("2006-01-02 15:04:05")
	updatedAt := time.Unix(server.UpdatedAt, 0).Format("2006-01-02 15:04:05")
	fmt.Printf("\nCreated At:      %s\n", createdAt)
	fmt.Printf("Updated At:      %s\n", updatedAt)

	return nil
}

func runNodeStat(store *sqlite.Store, id int64, recordType, days int) error {
	ctx := context.Background()

	// First verify the node exists
	server, err := store.Servers().FindByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return fmt.Errorf("node with ID %d not found", id)
		}
		return err
	}

	fmt.Printf("Statistics for Node: %s (ID: %d)\n", server.Name, server.ID)
	fmt.Printf("Record Type: %s, Days: %d\n", recordTypeLabel(recordType), days)
	fmt.Println("========================================")

	// Get stats from repository
	since := time.Now().AddDate(0, 0, -days).Unix()
	limit := days * 24 // hourly
	if recordType == 1 {
		limit = days // daily
	}

	records, err := store.StatServers().ListByServer(ctx, id, recordType, since, limit)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		fmt.Println("No statistics found for this period.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DATE\tUPLOAD\tDOWNLOAD\tTOTAL\tCPU\tMEM")

	var totalUp, totalDown int64
	for _, r := range records {
		date := time.Unix(r.RecordAt, 0).Format("2006-01-02 15:04")
		if recordType == 1 {
			date = time.Unix(r.RecordAt, 0).Format("2006-01-02")
		}
		memUsage := "N/A"
		if r.MemTotal > 0 {
			memUsage = fmt.Sprintf("%.1f%%", float64(r.MemUsed)/float64(r.MemTotal)*100)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.1f%%\t%s\n",
			date,
			formatBytes(r.Upload),
			formatBytes(r.Download),
			formatBytes(r.Upload+r.Download),
			r.CPUAvg,
			memUsage,
		)
		totalUp += r.Upload
		totalDown += r.Download
	}
	if err := w.Flush(); err != nil {
			slog.Warn("failed to flush node list output", "error", err)
		}

	fmt.Println("----------------------------------------")
	fmt.Printf("Total: ↑%s ↓%s = %s\n",
		formatBytes(totalUp),
		formatBytes(totalDown),
		formatBytes(totalUp+totalDown))

	return nil
}

func runNodeStatus(store *sqlite.Store) error {
	ctx := context.Background()
	servers, err := store.Servers().ListAll(ctx)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	var online, warning, offline int
	for _, s := range servers {
		if now-s.LastHeartbeatAt < 120 {
			online++
		} else if now-s.LastHeartbeatAt < 300 {
			warning++
		} else {
			offline++
		}
	}

	fmt.Println("Node Status Summary")
	fmt.Println("===================")
	fmt.Printf("\033[32m● Online:  %d\033[0m\n", online)
	fmt.Printf("\033[33m● Warning: %d\033[0m\n", warning)
	fmt.Printf("\033[31m● Offline: %d\033[0m\n", offline)
	fmt.Printf("  Total:   %d\n", len(servers))

	return nil
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds ago", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm ago", seconds/60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%dh ago", seconds/3600)
	}
	return fmt.Sprintf("%dd ago", seconds/86400)
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func recordTypeLabel(recordType int) string {
	if recordType == 0 {
		return "Hourly"
	}
	return "Daily"
}
