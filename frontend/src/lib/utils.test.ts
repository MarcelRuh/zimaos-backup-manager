import { describe, expect, it } from "vitest";
import { formatBytes, statusTone } from "./utils";

describe("utils", () => {
  it("formats bytes", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(2048)).toContain("KiB");
  });

  it("maps status tones", () => {
    expect(statusTone("completed")).toBe("ok");
    expect(statusTone("running")).toBe("run");
    expect(statusTone("failed")).toBe("err");
    expect(statusTone("mystery")).toBe("warn");
  });
});
