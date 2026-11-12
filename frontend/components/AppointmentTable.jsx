"use client"
const statusStyles = {
  confirmed: 'bg-emerald-100 text-emerald-800 border border-emerald-300',
  pending: 'bg-amber-100 text-amber-800 border border-amber-300',
  cancelled: 'bg-rose-100 text-rose-800 border border-rose-300',
  completed: 'bg-blue-100 text-blue-800 border border-blue-300',
  no_show: 'bg-purple-100 text-purple-800 border border-purple-300',
}

export default function AppointmentTable({
  appointments,
  onEdit,
  onCancel,
  onDelete,
  canEdit = true,
  canCancel = false,
  canDelete = true,
}) {
  const hasActions = canEdit || canCancel || canDelete

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
                  <span className={`px-2 py-0.5 rounded text-xs font-semibold uppercase tracking-wider ${
                    statusStyles[a.status] || 'bg-gray-100 text-gray-800'
                  }`}>
                    {a.status === 'no_show' ? 'No Show' : a.status}
                  </span>
                </Td>
                {hasActions && (
                  <Td>
                    <div className="flex gap-2">
                      {canEdit && (
                        <button
                          className="px-2 py-1 rounded bg-yellow-500 hover:bg-yellow-600 text-white text-xs font-medium"
                          onClick={() => onEdit(a)}
                        >
                          Edit
                        </button>
                      )}
                      {canCancel && a.status !== 'cancelled' && (
                        <button
                          className="px-2 py-1 rounded bg-orange-600 hover:bg-orange-700 text-white text-xs font-medium"
                          onClick={() => onCancel && onCancel(a)}
                        >
                          Cancel
                        </button>
                      )}
                      {canDelete && (
                        <button
                          className="px-2 py-1 rounded bg-red-600 hover:bg-red-700 text-white text-xs font-medium"
                          onClick={() => onDelete(a)}
                        >
                          Delete
                        </button>
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
