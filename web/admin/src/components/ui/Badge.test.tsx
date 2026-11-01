import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Badge } from "./Badge";

describe("Badge", () => {
  it("renders label", () => {
    render(<Badge tone="ok">Signaling online</Badge>);
    expect(screen.getByText("Signaling online")).toBeInTheDocument();
  });
});
