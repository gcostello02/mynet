/** Shell-safe `mynet join` command (bash/zsh single-quoted invitation). */
export function joinCommand(invitation: string, deviceName = "my-device"): string {
  const escaped = invitation.replace(/'/g, "'\\''");
  return `mynet join --name ${deviceName} '${escaped}'`;
}
