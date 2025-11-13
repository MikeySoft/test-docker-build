import React from "react";
import { useQuery } from "@tanstack/react-query";
import apiClient from "../api/client";

interface HostDockerVersionProps {
  hostId: string;
}

const HostDockerVersion: React.FC<HostDockerVersionProps> = ({ hostId }) => {
  const { data } = useQuery({
    queryKey: ["host-info", hostId],
    queryFn: () => apiClient.getHostInfo(hostId),
    refetchInterval: 60_000,
  });

  if (!data?.docker_version) return null;

  return <span>Docker {data.docker_version}</span>;
};

export default HostDockerVersion;


