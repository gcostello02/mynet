import { describe, expect, it } from "vitest";
import { readCSRFCookie } from "./csrf";

describe("readCSRFCookie", () => {
  it("reads mynet_csrf cookie", () => {
    document.cookie = "mynet_csrf=abc123; path=/";
    expect(readCSRFCookie()).toBe("abc123");
  });
});
