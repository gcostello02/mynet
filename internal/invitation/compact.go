package invitation

// Compact builds the canonical base64url(JSON) string consumed by `mynet join`.
func Compact(serverURL, networkID, coordinatorFingerprint, coordinatorPublicKey, enrollmentToken string) (string, error) {
	if networkID == "" {
		networkID = "default"
	}
	inv := Invite{
		Version:                Version,
		ServerURL:              serverURL,
		NetworkID:              networkID,
		CoordinatorFingerprint: coordinatorFingerprint,
		CoordinatorPublicKey:   coordinatorPublicKey,
		EnrollmentToken:        enrollmentToken,
	}
	return inv.EncodeCompact()
}
