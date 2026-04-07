import { useState, useEffect } from 'react';

interface PlatformUser {
  id: string;
  email: string;
  displayName: string;
  role: string;
}

export default function UserList() {
  const [users, setUsers] = useState<PlatformUser[]>([]);
  const [message, setMessage] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch('api/v1/users')
      .then(res => res.json())
      .then(data => {
        setUsers(data.users || []);
        setMessage(data.message || null);
        setLoading(false);
      })
      .catch(err => {
        setError(err.message);
        setLoading(false);
      });
  }, []);

  if (loading) return <div className="feature-card"><p>Loading tenant users...</p></div>;
  if (error) return <div className="feature-card"><p>Error: {error}</p></div>;

  return (
    <div className="feature-card">
      <h3>Tenant Users (Platform API)</h3>
      {message && <p style={{ color: '#888', fontSize: '0.85em' }}>{message}</p>}
      {users.length === 0 && !message && <p>No users found.</p>}
      {users.length > 0 && (
        <table style={{ width: '100%', borderCollapse: 'collapse', marginTop: '0.5rem' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid #333' }}>
              <th style={{ textAlign: 'left', padding: '0.5rem' }}>Name</th>
              <th style={{ textAlign: 'left', padding: '0.5rem' }}>Email</th>
              <th style={{ textAlign: 'left', padding: '0.5rem' }}>Role</th>
            </tr>
          </thead>
          <tbody>
            {users.map(u => (
              <tr key={u.id} style={{ borderBottom: '1px solid #222' }}>
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
