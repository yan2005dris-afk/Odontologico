/**
 * Global test setup (wi-14 S3).
 *
 * Registers the `@testing-library/jest-dom` matchers (`toBeInTheDocument`,
 * `toHaveTextContent`, ...) on Vitest's `expect`, so the component suites can
 * assert against the accessibility tree instead of against raw HTML strings.
 *
 * It also installs the browser APIs that jsdom does not implement but that the
 * Radix primitives (`Popover`, `DropdownMenu`, `Select`, `Tooltip`) rely on:
 * without them, opening a popover in a test throws `ResizeObserver is not
 * defined`.
 */

import "@testing-library/jest-dom/vitest";

if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = class ResizeObserverStub {
    observe(): void {}
    unobserve(): void {}
    disconnect(): void {}
  };
}

if (typeof Element !== "undefined") {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.setPointerCapture) {
    Element.prototype.setPointerCapture = () => {};
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }
}

if (typeof window !== "undefined" && typeof window.matchMedia !== "function") {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as typeof window.matchMedia;
}
