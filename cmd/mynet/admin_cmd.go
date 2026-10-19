package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/control"
	"github.com/grantcostello/mynet/internal/invitation"
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

func promptPassword(label string) string {
	fmt.Fprint(os.Stderr, label)
	r := bufio.NewReader(os.Stdin)
	b, _ := r.ReadString('\n')
	return strings.TrimSpace(b)
}

func adminUsage() {
	fmt.Println(`Admin commands (credentials in admin-session.json, not device session):

  mynet admin login --server <https-url> [--user <name>]
  mynet admin logout
  mynet admin status
  mynet admin devices list [--format json]
  mynet admin devices revoke <device_id>
  mynet admin invitations list
  mynet admin invitations create [--label <name>] [--expires 86400] [--max-uses 1]`)
}
