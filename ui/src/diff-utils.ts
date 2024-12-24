import { OperationMessage } from "./types";
import { v4 as uuidv4 } from "uuid";

/**
 * Finds the longest common prefix.
 */
function findLongestCommonPrefix(a: string, b: string): number {
  let i = 0;
  const minLen = Math.min(a.length, b.length);
  while (i < minLen && a[i] === b[i]) {
    i++;
  }
  return i;
}

/**
 * Finds the longest common suffix, given a prefix boundary.
 */
function findLongestCommonSuffix(
  a: string,
  b: string,
  prefixLen: number,
): number {
  let i = 0;
  const aLen = a.length - prefixLen;
  const bLen = b.length - prefixLen;
  while (i < aLen && i < bLen && a[a.length - 1 - i] === b[b.length - 1 - i]) {
    i++;
  }
  return i;
}

/**
 * generateDiffOperations
 * Returns an array of OperationMessages transforming oldVal to newVal.
 * We allow multi-character "insert" or "delete" to reduce spam.
 */
export function generateDiffOperations(
  oldVal: string,
  newVal: string,
  docId: string,
  userID: string,
): OperationMessage[] {
  const ops: OperationMessage[] = [];
  if (oldVal === newVal) return ops;

  const prefixLen = findLongestCommonPrefix(oldVal, newVal);
  const suffixLen = findLongestCommonSuffix(oldVal, newVal, prefixLen);

  const oldMid = oldVal.slice(prefixLen, oldVal.length - suffixLen);
  const newMid = newVal.slice(prefixLen, newVal.length - suffixLen);

  if (oldMid.length > 0) {
    ops.push({
      type: "delete",
      docId,
      position: prefixLen,
      value: oldMid,
      operationId: uuidv4(),
      source: userID,
      timestamp: Date.now(),
    });
  }

  if (newMid.length > 0) {
    ops.push({
      type: "insert",
      docId,
      position: prefixLen,
      value: newMid,
      operationId: uuidv4(),
      source: userID,
      timestamp: Date.now(),
    });
  }

  return ops;
}
