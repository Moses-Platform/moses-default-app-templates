import { useUsers } from '../api/hooks';

export default function UserList() {
  const { data, isPending, isError, error } = useUsers();
  const users = data?.users ?? [];
  const message = data?.message ?? null;

  if (isPending) return <div className="feature-card"><p>Loading tenant users...</p></div>;
  if (isError) return <div className="feature-card"><p>Error: {(error as Error).message}</p></div>;

  return (
    <div className="feature-card">
      <h3>Tenant Users (Platform API)</h3>
      {message && <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.85em' }}>{message}</p>}
      {users.length === 0 && !message && <p>No users found.</p>}
      {users.length > 0 && (
        <table style={{ width: '100%', borderCollapse: 'collapse', marginTop: '0.5rem' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--color-border)' }}>
              <th style={{ textAlign: 'left', padding: '0.5rem' }}>Name</th>
              <th style={{ textAlign: 'left', padding: '0.5rem' }}>Email</th>
              <th style={{ textAlign: 'left', padding: '0.5rem' }}>Role</th>
            </tr>
          </thead>
          <tbody>
            {users.map(u => (
              <tr key={u.id} style={{ borderBottom: '1px solid var(--color-border)' }}>
                <td style={{ padding: '0.5rem' }}>{u.displayName}</td>
                <td style={{ padding: '0.5rem' }}>{u.email}</td>
                <td style={{ padding: '0.5rem' }}>{u.role}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
