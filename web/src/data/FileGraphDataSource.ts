import type { FileGraphSnapshot } from "../types/fileGraph"

export interface FileGraphDataSource {
  load(): Promise<FileGraphSnapshot>
}
