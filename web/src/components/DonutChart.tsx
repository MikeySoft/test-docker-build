import React from "react";

interface DonutChartProps {
  size?: number; // px
  strokeWidth?: number; // px
  value: number; // 0-100
  color?: string;
  trackColor?: string;
  centerText?: string;
  footerLabel?: string;
}

export const DonutChart: React.FC<DonutChartProps> = ({
  size = 64,
  strokeWidth = 8,
  value,
  color = "#10b981",
  trackColor = "#e5e7eb",
  centerText,
  footerLabel,
}) => {
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const clamped = Math.max(0, Math.min(100, value));
  const offset = circumference - (clamped / 100) * circumference;

  return (
    <div className="flex flex-col items-center gap-1">
      <svg width={size} height={size} className="flex-shrink-0" role="img" aria-label={footerLabel ?? "donut chart"}>
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="transparent"
          stroke={trackColor}
          strokeWidth={strokeWidth}
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="transparent"
          stroke={color}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          transform={`rotate(-90 ${size / 2} ${size / 2})`}
        />
        <text x="50%" y="50%" dominantBaseline="middle" textAnchor="middle" fontSize={12} fill="#374151">
          {centerText ?? `${Math.round(clamped)}%`}
        </text>
      </svg>
      {footerLabel && (
        <div className="text-xs text-gray-600 dark:text-gray-400 text-center leading-tight">
          {footerLabel}
        </div>
      )}
    </div>
  );
};

export default DonutChart;


