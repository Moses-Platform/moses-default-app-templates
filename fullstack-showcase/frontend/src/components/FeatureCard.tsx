import { ReactNode } from 'react';
import './FeatureCard.css';

interface FeatureCardProps {
  icon?: string;
  title: string;
  description: string;
  details?: string[];
  children?: ReactNode;
  onClick?: () => void;
}

export default function FeatureCard({
  icon,
  title,
  description,
  details,
  children,
  onClick,
}: FeatureCardProps) {
  const CardWrapper = onClick ? 'button' : 'div';

  return (
    <CardWrapper
      className={`feature-card ${onClick ? 'clickable' : ''}`}
      onClick={onClick}
      {...(onClick && { type: 'button' })}
    >
      {icon && <div className="feature-icon">{icon}</div>}
      <h3 className="feature-title">{title}</h3>
      <p className="feature-description">{description}</p>
      {details && details.length > 0 && (
        <ul className="feature-details">
          {details.map((detail, index) => (
            <li key={index}>{detail}</li>
          ))}
        </ul>
      )}
      {children}
    </CardWrapper>
  );
}
