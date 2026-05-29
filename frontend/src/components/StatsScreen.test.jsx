import { render, screen } from '@testing-library/react';
import StatsScreen from './StatsScreen';

describe('StatsScreen', () => {
  it('renders advanced operational metrics and tables', () => {
    render(
      <StatsScreen
        locale="en"
        stats={{
          archivedPatients: 3,
          totalConsultations: 7,
          avgConsultationMinutes: 12.4,
          avgWaitToTriageMinutes: 8.7,
          avgWaitToAssignMinutes: 15.2,
          triageSlaBreaches: 2,
          alertBeds: 1,
          triageByLevel: { 0: 1, 1: 2, 2: 3, 3: 4, 4: 5 },
          consultationsByDate: [{ date: '2026-05-29', count: 4 }],
          consultationsByHour: [{ hour: '14:00', count: 2 }],
          patientsByStatus: { triaged: 2, assigned: 1 },
          patientsByType: { medical: 2, traumato: 1 },
        }}
      />,
    );

    expect(screen.getByText('Statistics')).toBeInTheDocument();
    expect(screen.getByText('Avg wait to triage (min)')).toBeInTheDocument();
    expect(screen.getByText('Triage SLA breaches')).toBeInTheDocument();
    expect(screen.getByText('Consultations by hour')).toBeInTheDocument();
    expect(screen.getByText('Patients by status')).toBeInTheDocument();
    expect(screen.getByText('Patients by type')).toBeInTheDocument();
    expect(screen.getByText('14:00')).toBeInTheDocument();
    expect(screen.getByText('triaged')).toBeInTheDocument();
    expect(screen.getByText('medical')).toBeInTheDocument();
  });
});
