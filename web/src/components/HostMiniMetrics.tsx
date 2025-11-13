import React from "react";
import { useQuery } from "@tanstack/react-query";
import apiClient from "../api/client";
import DonutChart from "./DonutChart";

interface HostMiniMetricsProps {
  hostId: string;
}

export const HostMiniMetrics: React.FC<HostMiniMetricsProps> = ({ hostId }) => {
  const { data: info } = useQuery({
    queryKey: ["host-info", hostId],
    queryFn: () => apiClient.getHostInfo(hostId),
    refetchInterval: 60_000,
  });

  // We only need capacities like Host Metrics tab: cores, total memory, total disk
  if (!info) return null;

  return (
    <div className="mt-3 grid grid-cols-3 gap-3">
      <DonutChart value={100} color="#3b82f6" centerText={`${info.ncpu ?? '—'}`} footerLabel="CPU Cores" />
      <DonutChart value={100} color="#10b981" centerText={`${formatBytes(info.mem_total)}`} footerLabel="Memory" />
      <DonutChart value={100} color="#ef4444" centerText={`${formatBytes(info.disk_total ?? 0)}`} footerLabel="Disk" />
    </div>
  );
};

export default HostMiniMetrics;

function formatBytes(bytes?: number) {
  if (!bytes || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let val = bytes;
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024;
    i++;
  }
  return `${val.toFixed(1)} ${units[i]}`;
}


