import { beforeEach, describe, expect, it } from "vitest";
import type * as Sentry from "@sentry/react";
import { allowEvent, resetEventBudget } from "../sentry";

function errorEvent(type: string, value: string): Sentry.ErrorEvent {
	return {
		type: undefined,
		exception: { values: [{ type, value }] },
	} as unknown as Sentry.ErrorEvent;
}

describe("allowEvent (client-side quota guard)", () => {
	beforeEach(() => {
		resetEventBudget();
	});

	it("mutes duplicate signatures within the dedup window", () => {
		const now = Date.now();
		expect(allowEvent(errorEvent("ErrX", "boom"), now)).toBe(true);
		expect(allowEvent(errorEvent("ErrX", "boom"), now + 60_000)).toBe(false);
		// After the 10min dedup window the same signature may send again.
		expect(allowEvent(errorEvent("ErrX", "boom"), now + 10 * 60_000 + 1)).toBe(
			true,
		);
	});

	it("caps distinct errors at the hourly budget", () => {
		const now = Date.now();
		for (let i = 0; i < 5; i++) {
			expect(allowEvent(errorEvent("Err", `distinct-${i}`), now)).toBe(true);
		}
		expect(allowEvent(errorEvent("Err", "one-too-many"), now)).toBe(false);
		// A new hourly window resets the budget.
		expect(
			allowEvent(errorEvent("Err", "one-too-many"), now + 60 * 60_000 + 1),
		).toBe(true);
	});

	it("falls back to message fingerprint when there is no exception", () => {
		const now = Date.now();
		const msgEvent = { message: "plain" } as unknown as Sentry.ErrorEvent;
		expect(allowEvent(msgEvent, now)).toBe(true);
		expect(allowEvent(msgEvent, now + 1)).toBe(false);
	});
});
