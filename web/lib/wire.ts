// Runtime guards for wire values that the mirrored types declare as present.
//
// lib/types.ts mirrors Go structs and TypeScript checks nothing here: the data
// arrives as JSON at runtime. Go marshals a nil slice as null, several of the
// columns behind these fields are nullable jsonb, and the BFF maps a NULL
// column to undefined. So a field typed `string[]` can arrive as null and a
// field typed `number` can arrive as null, and calling .join, .map or .toFixed
// on it throws the whole page away. Every component reads a list, a figure or a
// label through this file.
//
// This file must not substitute a value. A missing count is not zero and a
// missing label is not "unknown": each guard reports absence and the caller
// decides what sentence to print instead.

/** The array, with any null or undefined entry dropped. Never null. */
export function listOf<T>(value: readonly (T | null | undefined)[] | null | undefined): T[] {
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is T => item !== null && item !== undefined);
}

/**
 * The object, or an empty one. Arrays are not objects for this purpose.
 *
 * It takes `unknown` because the shapes it guards are partial records whose
 * declared value type already includes undefined, and a caller should not have
 * to cast its way past that to ask whether an object arrived at all. Every
 * value read out of the result goes through `numberOf` or `textOf`.
 */
export function mapOf<V>(value: unknown): Record<string, V | undefined> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return {};
  return value as Record<string, V | undefined>;
}

/** The string when it carries something, null when it is absent or blank. */
export function textOf(value: string | null | undefined): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

/** The number when it is a real one. NaN and Infinity are not measurements. */
export function numberOf(value: number | null | undefined): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}
