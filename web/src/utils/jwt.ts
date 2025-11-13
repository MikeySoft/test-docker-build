/**
 * Utility to decode JWT token (without verification - for display purposes only)
 * Since we trust tokens from our own server, we can safely decode for display
 */
export function decodeJWT(
  token: string
): { username?: string; role?: string; sub?: string } | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;

    // Decode the payload (second part)
    const payload = parts[1];
    // Add padding if needed
    const paddedPayload = payload + "=".repeat((4 - (payload.length % 4)) % 4);
    const decoded = atob(paddedPayload);
    const claims = JSON.parse(decoded);

    return {
      username: claims.username,
      role: claims.role,
      sub: claims.sub,
    };
  } catch (error) {
    console.error("Failed to decode JWT:", error);
    return null;
  }
}
