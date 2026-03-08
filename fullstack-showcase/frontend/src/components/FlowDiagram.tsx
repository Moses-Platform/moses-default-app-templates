import './FlowDiagram.css';

interface FlowStep {
  label: string;
  icon?: string;
  description?: string;
}

interface FlowDiagramProps {
  steps: FlowStep[];
  title?: string;
}

export default function FlowDiagram({ steps, title }: FlowDiagramProps) {
  return (
    <div className="flow-diagram">
      {title && <h4 className="flow-title">{title}</h4>}
      <div className="flow-steps">
        {steps.map((step, index) => (
          <div key={index} className="flow-step">
            <div className="flow-step-content">
              {step.icon && <div className="flow-step-icon">{step.icon}</div>}
              <div className="flow-step-label">{step.label}</div>
              {step.description && (
                <div className="flow-step-description">{step.description}</div>
              )}
            </div>
            {index < steps.length - 1 && <div className="flow-arrow">→</div>}
          </div>
        ))}
      </div>
    </div>
  );
}
