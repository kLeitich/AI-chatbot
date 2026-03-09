import Chatbot from '../../../components/Chatbot'

export default function Page({ params }) {
  const { tenant } = params
  return <Chatbot tenant={tenant} />
}

