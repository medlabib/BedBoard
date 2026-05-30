import { fireEvent, render, screen } from '@testing-library/react';
import AccountScreen from './AccountScreen';
import PatientViewPage from './PatientViewPage';
import ReceptionPage from './ReceptionPage';
import SimpleModal from './SimpleModal';

describe('Basic components coverage', () => {
  it('renders AccountScreen and updates password fields', () => {
    const setPasswordForm = vi.fn();
    const changeOwnPassword = vi.fn();

    render(
      <AccountScreen
        passwordForm={{ currentPassword: '', newPassword: '', confirmPassword: '' }}
        setPasswordForm={setPasswordForm}
        changeOwnPassword={changeOwnPassword}
        user={{ username: 'admin' }}
        locale="en"
      />,
    );

    expect(screen.getByText('Change my password')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Current password'), { target: { value: 'old' } });
    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'new' } });
    fireEvent.change(screen.getByLabelText('Confirm'), { target: { value: 'new' } });
    fireEvent.click(screen.getByText('Update'));

    expect(setPasswordForm).toHaveBeenCalled();
    expect(changeOwnPassword).toHaveBeenCalledTimes(1);
  });

  it('renders ReceptionPage and fires action callbacks', () => {
    const logout = vi.fn();
    const openPatientPage = vi.fn();

    render(
      <ReceptionPage
        logout={logout}
        openPatientPage={openPatientPage}
        locale="en"
        brandName="BedBoard"
        brandLogo=""
      />,
    );

    expect(screen.getByText('BedBoard - Reception')).toBeInTheDocument();
    fireEvent.click(screen.getByText('Open patient page'));
    fireEvent.click(screen.getByText('Logout'));

    expect(openPatientPage).toHaveBeenCalledTimes(1);
    expect(logout).toHaveBeenCalledTimes(1);
  });

  it('renders PatientViewPage and SimpleModal interactions', () => {
    const openMainPage = vi.fn();
    const onCancel = vi.fn();
    const onConfirm = vi.fn();

    render(
      <>
        <PatientViewPage
          openMainPage={openMainPage}
          patientPanel={<div>Panel</div>}
          locale="en"
          brandName="BedBoard"
          brandLogo=""
        />
        <SimpleModal
          title="Confirm"
          text="Proceed?"
          onCancel={onCancel}
          onConfirm={onConfirm}
          locale="en"
        />
      </>,
    );

    expect(screen.getByText('BedBoard - Room view')).toBeInTheDocument();
    expect(screen.getByText('Panel')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Back to board' }));
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    fireEvent.click(screen.getByRole('button', { name: 'Confirm' }));

    expect(openMainPage).toHaveBeenCalledTimes(1);
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});
