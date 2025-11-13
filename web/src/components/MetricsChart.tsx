import { useMemo } from "react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";

interface DataPoint {
  timestamp: string;
  value: number;
}

interface MetricsChartProps {
  data: DataPoint[];
  title: string;
  unit?: string;
  color?: string;
  height?: number;
}

export function MetricsChart({
  data,
  title,
  unit = "",
  color = "#3b82f6",
  height = 300,
}: MetricsChartProps) {
  // Transform data for Recharts
  const chartData = useMemo(() => {
    return data.map((point) => ({
      time: new Date(point.timestamp).toLocaleTimeString(),
      timestamp: point.timestamp,
      value: Math.round(point.value * 100) / 100, // Round to 2 decimal places
    }));
  }, [data]);

  if (chartData.length === 0) {
    return (
      <div className="flex items-center justify-center h-64 border border-gray-200 rounded-lg bg-gray-50">
        <p className="text-gray-500">No metrics data available</p>
      </div>
    );
  }

  const formatTooltipValue = (value: number) => {
    if (unit === "%") {
      return `${value.toFixed(2)}%`;
    }
    if (unit === "bytes") {
      return formatBytes(value);
    }
    return `${value} ${unit}`;
  };

  return (
    <div className="w-full">
      <h3 className="text-sm font-medium text-gray-700 mb-2">{title}</h3>
      <ResponsiveContainer width="100%" height={height}>
        <LineChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" className="stroke-gray-200" />
          <XAxis
            dataKey="time"
            className="text-xs text-gray-600"
            tick={{ fill: "#6b7280" }}
          />
          <YAxis
            className="text-xs text-gray-600"
            tick={{ fill: "#6b7280" }}
            tickFormatter={(value) => {
              if (unit === "bytes") {
                return formatBytes(value);
              }
              if (unit === "%") {
                return `${value}%`;
              }
              return value.toString();
            }}
          />
          <Tooltip
            contentStyle={{
              backgroundColor: "white",
              border: "1px solid #e5e7eb",
              borderRadius: "6px",
            }}
            formatter={(value: number) => formatTooltipValue(value)}
            labelFormatter={(label) => {
              const point = chartData.find((d) => d.time === label);
              return point
                ? new Date(point.timestamp).toLocaleString()
                : label;
            }}
          />
          <Legend />
          <Line
            type="monotone"
            dataKey="value"
            stroke={color}
            strokeWidth={2}
            dot={false}
            name={title}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + " " + sizes[i];
}

