import { fireEvent, render, screen } from '@testing-library/react';
import PatientsRows from './PatientsRows';

describe('PatientsRows', () => {
  it('renders status column and loads timeline on row click', () => {
    const setSelectedPatientReg = vi.fn();
    const refreshPatientEvents = vi.fn().mockResolvedValue(undefined);
    const setConfirm = vi.fn();
    const setPatients = vi.fn();
    const setScreen = vi.fn();
    const setNewPatient = vi.fn();

    render(
      <table>
        <tbody>
          <PatientsRows
            activePatients={[
              {
                registrationNumber: 'P-001',
                name: 'PATIENT-001',
                patientType: 'medical',
                triageScore: 4,
                status: 'triaged',
                roomName: 'Room 1',
                bedName: '',
                bedNumber: null,
              },
            ]}
            canViewTriage
            canViewPatientType
            authenticated
            canManageBeds
            canArchivePatients
            setScreen={setScreen}
            setNewPatient={setNewPatient}
            setConfirm={setConfirm}
            api={vi.fn()}
            showError={vi.fn()}
            showSuccess={vi.fn()}
            readErrorMessage={vi.fn()}
            setPatients={setPatients}
            setSelectedPatientReg={setSelectedPatientReg}
            refreshPatientEvents={refreshPatientEvents}
            escapeText={(v) => String(v ?? '')}
            locale="en"
          />
        </tbody>
      </table>,
    );

    expect(screen.getByText('triaged')).toBeInTheDocument();

    fireEvent.click(screen.getByText('P-001'));

    expect(setSelectedPatientReg).toHaveBeenCalledWith('P-001');
    expect(refreshPatientEvents).toHaveBeenCalledWith('P-001');
  });
});
