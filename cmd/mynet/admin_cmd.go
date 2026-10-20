package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/invitation"
)

func cmdAdmin(args []string) {
	if len(args) < 1 {
		adminUsage()
		os.Exit(2)
	}
	sd := stateDir()
	switch args[0] {
	case "login":
		fs := flag.NewFlagSet("admin login", flag.ExitOnError)
		server := fs.String("server", "", "coordinator https URL")
		user := fs.String("user", "", "admin username")
		_ = fs.Parse(args[1:])
		if *server == "" {
			fmt.Fprintln(os.Stderr, "--server is required (https://...)")
			os.Exit(2)
		}
		pass := promptPassword("Admin password: ")
		username := *user
		if username == "" {
			fmt.Fprint(os.Stderr, "Username: ")
			fmt.Scanln(&username)
		}
		api, err := control.NewSecureAPIClient(*server)
		if err != nil {
			fatal(err)
		}
		resp, err := api.AdminLogin(username, pass)
		if err != nil {
			fatal(err)
		}
		if err := config.SaveAdminSession(sd, config.AdminSession{
			ServerURL:    *server,
			SessionToken: resp.SessionToken,
			ExpiresAt:    resp.ExpiresAt,
		}); err != nil {
			fatal(err)
		}
		fmt.Println("Admin session saved (separate from device credentials).")
	case "setup":
		fs := flag.NewFlagSet("admin setup", flag.ExitOnError)
		server := fs.String("server", "http://127.0.0.1:8080", "coordinator URL (loopback on host, or https with --bootstrap-token)")
		user := fs.String("user", "admin", "first admin username")
		network := fs.String("network", "MyNet", "network display name")
		publicURL := fs.String("public-url", "", "HTTPS URL clients use (required)")
		bootstrap := fs.String("bootstrap-token", "", "one-time token from mynet admin bootstrap (required for remote https)")
		loginAfter := fs.Bool("login", true, "save admin session after setup (https public-url only)")
		_ = fs.Parse(args[1:])
		if *publicURL == "" {
			fmt.Fprintln(os.Stderr, "--public-url is required (https://...)")
			os.Exit(2)
		}
		pass := promptPassword("Admin password (min 12 chars): ")
		resp, err := control.AdminSetup(*server, *bootstrap, control.AdminSetupRequest{
			Username:           *user,
			Password:           pass,
			NetworkDisplayName: *network,
			PublicServerURL:    *publicURL,
		})
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Coordinator initialized.\n  fingerprint: %s\n  admin_id: %s\n", resp.CoordinatorFingerprint, resp.AdminID)
		if *loginAfter && strings.HasPrefix(strings.ToLower(*publicURL), "https://") {
			api, err := control.NewSecureAPIClient(*publicURL)
			if err != nil {
				fmt.Fprintln(os.Stderr, "setup ok; admin login skipped:", err)
				break
			}
			loginResp, err := api.AdminLogin(*user, pass)
			if err != nil {
				fmt.Fprintln(os.Stderr, "setup ok; admin login failed:", err)
				break
			}
			if err := config.SaveAdminSession(sd, config.AdminSession{
				ServerURL:    *publicURL,
				SessionToken: loginResp.SessionToken,
				ExpiresAt:    loginResp.ExpiresAt,
			}); err != nil {
				fatal(err)
			}
			fmt.Println("Admin session saved.")
		} else if *loginAfter {
			fmt.Println("Run: mynet admin login --server", *publicURL)
		}
	case "bootstrap":
		fs := flag.NewFlagSet("admin bootstrap", flag.ExitOnError)
		server := fs.String("server", "http://127.0.0.1:8080", "coordinator URL (must be loopback on the host)")
		_ = fs.Parse(args[1:])
		resp, err := http.Get(strings.TrimRight(*server, "/") + "/v1/setup/bootstrap")
		if err != nil {
			fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fatal(fmt.Errorf("bootstrap fetch failed: http %d", resp.StatusCode))
		}
		var out map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	case "logout":
		if err := config.ClearAdminSession(sd); err != nil {
			fatal(err)
		}
		fmt.Println("Admin session cleared.")
	case "status":
		api, err := loadAdminAPI(sd)
		if err != nil {
			fatal(err)
		}
		st, err := api.AdminStatus()
		if err != nil {
			fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(st)
	case "devices":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: mynet admin devices list|revoke <device_id>")
			os.Exit(2)
		}
		api, err := loadAdminAPI(sd)
		if err != nil {
			fatal(err)
		}
		switch args[1] {
		case "list":
			fs := flag.NewFlagSet("admin devices list", flag.ExitOnError)
			format := fs.String("format", "json", "output format: json")
			_ = fs.Parse(args[2:])
			devs, err := api.AdminListDevices()
			if err != nil {
				fatal(err)
			}
			enc := json.NewEncoder(os.Stdout)
			if *format == "json" {
				enc.SetIndent("", "  ")
			}
			_ = enc.Encode(devs)
		case "revoke":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "device_id required")
				os.Exit(2)
			}
			if err := api.AdminRevokeDevice(args[2]); err != nil {
				fatal(err)
			}
			fmt.Fprintln(os.Stderr, "Device revoked.")
		default:
			adminUsage()
			os.Exit(2)
		}
	case "password":
		if len(args) < 2 || args[1] != "change" {
			fmt.Fprintln(os.Stderr, "usage: mynet admin password change")
			os.Exit(2)
		}
		api, err := loadAdminAPI(sd)
		if err != nil {
			fatal(err)
		}
		current := promptPassword("Current password: ")
		newPass := promptPassword("New password (min 12 chars): ")
		confirm := promptPassword("Confirm new password: ")
		if newPass != confirm {
			fatal(fmt.Errorf("new passwords do not match"))
		}
		if len(newPass) < 12 {
			fatal(fmt.Errorf("new password must be at least 12 characters"))
		}
		if err := api.AdminChangePassword(current, newPass); err != nil {
			fatal(err)
		}
		if err := config.ClearAdminSession(sd); err != nil {
			fatal(err)
		}
		fmt.Println("Password changed. All admin sessions were invalidated.")
		fmt.Println("Run: mynet admin login --server <https-url>")
	case "invitations":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: mynet admin invitations create")
			os.Exit(2)
		}
		api, err := loadAdminAPI(sd)
		if err != nil {
			fatal(err)
		}
		switch args[1] {
		case "list":
			invites, err := api.AdminListInvitations()
			if err != nil {
				fatal(err)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(invites)
		case "create":
			fs := flag.NewFlagSet("admin invitations create", flag.ExitOnError)
			label := fs.String("label", "", "invitation label")
			exp := fs.Int("expires", 86400, "expiry seconds")
			uses := fs.Int("max-uses", 1, "max uses")
			_ = fs.Parse(args[2:])
			resp, err := api.AdminCreateInvitation(*label, *exp, *uses)
			if err != nil {
				fatal(err)
			}
			sess, _ := config.LoadAdminSession(sd)
			inv := invitation.Invite{
				Version:                invitation.Version,
				ServerURL:              sess.ServerURL,
				NetworkID:              "default",
				CoordinatorFingerprint: resp["coordinator_fingerprint"],
				CoordinatorPublicKey:   "",
				EnrollmentToken:        resp["enrollment_token"],
			}
			if pubAPI, err := control.NewSecureAPIClient(sess.ServerURL); err == nil {
				if pub, err := pubAPI.NetworkPublic(); err == nil {
					inv.NetworkID = pub.NetworkID
					inv.CoordinatorPublicKey = pub.CoordinatorPublicKey
				}
			}
			compact, err := inv.EncodeCompact()
			if err != nil {
				fatal(err)
			}
			fmt.Println("Invitation created (treat as secret; deliver out-of-band):")
			fmt.Println(compact)
		default:
			adminUsage()
			os.Exit(2)
		}
	default:
		adminUsage()
		os.Exit(2)
	}
}

func loadAdminAPI(stateDir string) (*control.APIClient, error) {
	sess, err := config.LoadAdminSession(stateDir)
	if err != nil {
		return nil, fmt.Errorf("admin not logged in (mynet admin login): %w", err)
	}
	api, err := control.NewSecureAPIClient(sess.ServerURL)
	if err != nil {
		return nil, err
	}
	api.Token = sess.SessionToken
	return api, nil
}

func adminUsage() {
	fmt.Println(`Admin commands (credentials in admin-session.json, not device session):

  mynet admin setup --public-url https://coordinator.example [--server http://127.0.0.1:8080]
  mynet admin bootstrap --server http://127.0.0.1:8080
  mynet admin login --server <https-url> [--user <name>]
  mynet admin logout
  mynet admin status
  mynet admin devices list [--format json]
  mynet admin devices revoke <device_id>
  mynet admin invitations list
  mynet admin invitations create [--label <name>] [--expires 86400] [--max-uses 1]
  mynet admin password change`)
}
