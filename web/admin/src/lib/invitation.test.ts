import { describe, expect, it } from "vitest";
import { joinCommand } from "./invitation";

describe("joinCommand", () => {
  it("formats executable join with device name", () => {
    const cmd = joinCommand("eyJ2IjoxfQ", "minipc");
    expect(cmd).toBe("mynet join --name minipc 'eyJ2IjoxfQ'");
  });

  it("escapes single quotes in invitation", () => {
    const cmd = joinCommand("ab'cd", "laptop");
    expect(cmd).toBe("mynet join --name laptop 'ab'\\''cd'");
  });
});
