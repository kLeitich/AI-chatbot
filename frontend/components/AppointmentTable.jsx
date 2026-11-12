"use client"
export default function AppointmentTable({ appointments, onEdit, onDelete, canEdit = true, canDelete = true }) {
  const hasActions = canEdit || canDelete

  return (
    <div className="overflow-x-auto border rounded bg-white shadow">
      <table className="min-w-full text-sm">
        <thead className="bg-gray-100">
          <tr>
            <Th>ID</Th>
            <Th>Patient</Th>
            <Th>Doctor</Th>
            <Th>Date</Th>
            <Th>Time</Th>
            <Th>Reason</Th>
            <Th>Status</Th>
            {hasActions && <Th>Actions</Th>}
          </tr>
        </thead>
        <tbody>
          {appointments.length === 0 ? (
            <tr>
              <td colSpan={hasActions ? 8 : 7} className="p-4 text-center text-gray-500">
                No appointments found.
              </td>
            </tr>
          ) : (
            appointments.map((a) => (
              <tr key={a.id} className="border-t">
                <Td>{a.id}</Td>
                <Td>{a.patient_name}</Td>
                <Td>{a.doctor}</Td>
                <Td>{a.date}</Td>
                <Td>{a.time}</Td>
                <Td>{a.reason}</Td>
                <Td>
                  <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                    a.status === 'confirmed' ? 'bg-green-100 text-green-800' :
                    a.status === 'cancelled' ? 'bg-red-100 text-red-800' :
                    'bg-yellow-100 text-yellow-800'
                  }`}>
                    {a.status}
                  </span>
                </Td>
                {hasActions && (
                  <Td>
                    <div className="flex gap-2">
                      {canEdit && (
                        <button className="px-2 py-1 rounded bg-yellow-500 hover:bg-yellow-600 text-white" onClick={() => onEdit(a)}>Edit</button>
                      )}
                      {canDelete && (
                        <button className="px-2 py-1 rounded bg-red-600 hover:bg-red-700 text-white" onClick={() => onDelete(a)}>Delete</button>
                      )}
                    </div>
                  </Td>
                )}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}

function Th({ children }) { return <th className="text-left p-2 whitespace-nowrap">{children}</th> }
function Td({ children }) { return <td className="p-2 align-top whitespace-nowrap">{children}</td> }
