import React from 'react';

const FlotillaLogo: React.FC<{ className?: string }> = ({ className = 'h-8 w-8' }) => {
  return (
    <svg
      width="60"
      height="60"
      viewBox="0 0 60 60"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
    >
      {/* Row 1 */}
      <rect x="15" y="15" width="10" height="10" fill="#00d4ff" rx="2"/>
      <rect x="27" y="15" width="10" height="10" fill="#00d4ff" opacity="0.7" rx="2"/>
      <rect x="39" y="15" width="10" height="10" fill="#00d4ff" opacity="0.4" rx="2"/>

      {/* Row 2 */}
      <rect x="15" y="27" width="10" height="10" fill="#00d4ff" opacity="0.7" rx="2"/>
      <rect x="27" y="27" width="10" height="10" fill="#00d4ff" rx="2"/>
      <rect x="39" y="27" width="10" height="10" fill="#00d4ff" opacity="0.7" rx="2"/>

      {/* Row 3 */}
      <rect x="15" y="39" width="10" height="10" fill="#00d4ff" opacity="0.4" rx="2"/>
      <rect x="27" y="39" width="10" height="10" fill="#00d4ff" opacity="0.7" rx="2"/>
      <rect x="39" y="39" width="10" height="10" fill="#00d4ff" rx="2"/>
    </svg>
  );
};

export default FlotillaLogo;

