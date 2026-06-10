import AdminLogin from '../../../../components/AdminLogin'

export default function Page({ params }) {
  const { tenant } = params
  return <AdminLogin tenant={tenant} />
}
