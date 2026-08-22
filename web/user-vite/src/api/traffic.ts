import { api } from "./client";
import type { TrafficLog } from "@/types";

export async function fetchTrafficLogs(): Promise<TrafficLog[]> {
  const response = await api.get<TrafficLog[]>("/user/stat/getTrafficLog");
  return response.data;
}
